package security

import (
     "bytes"
//     "errors"
     "io"
     "log"
     "os"
     "path/filepath"
     "strings"
     "time"

     "crypto"
     "crypto/rand"
     "crypto/rsa"

     "github.com/dedis/protobuf"
     "github.com/Swarali/Peerster/message"
     "golang.org/x/crypto/openpgp"
     "golang.org/x/crypto/openpgp/armor"
     "golang.org/x/crypto/openpgp/packet"
)

var PRIVATE_KEY_FILE string
var PUBLIC_KEY_FILE string
var PUBLIC_KEY_TEXT string
var SEC_ENTITY *openpgp.Entity
var TRUSTED_PUBLIC_KEYS map[string]string

func createEntityFromKeys(pubKey *packet.PublicKey, privKey *packet.PrivateKey) *openpgp.Entity {
	config := packet.Config{
		DefaultHash:            crypto.SHA256,
		DefaultCipher:          packet.CipherAES256,
		DefaultCompressionAlgo: packet.CompressionZLIB,
		CompressionConfig: &packet.CompressionConfig{
			Level: 9,
		},
		RSABits: 4096,
	}
	currentTime := config.Now()
	uid := packet.NewUserId("", "", "")

	e := openpgp.Entity{
		PrimaryKey: pubKey,
		PrivateKey: privKey,
		Identities: make(map[string]*openpgp.Identity),
	}
	isPrimaryId := false

	e.Identities[uid.Id] = &openpgp.Identity{
		Name:   uid.Name,
		UserId: uid,
		SelfSignature: &packet.Signature{
			CreationTime: currentTime,
			SigType:      packet.SigTypePositiveCert,
			PubKeyAlgo:   packet.PubKeyAlgoRSA,
			Hash:         config.Hash(),
			IsPrimaryId:  &isPrimaryId,
			FlagsValid:   true,
			FlagSign:     true,
			FlagCertify:  true,
			IssuerKeyId:  &e.PrimaryKey.KeyId,
		},
	}

	keyLifetimeSecs := uint32(86400 * 365)

	e.Subkeys = make([]openpgp.Subkey, 1)
	e.Subkeys[0] = openpgp.Subkey{
		PublicKey: pubKey,
		PrivateKey: privKey,
		Sig: &packet.Signature{
			CreationTime:              currentTime,
			SigType:                   packet.SigTypeSubkeyBinding,
			PubKeyAlgo:                packet.PubKeyAlgoRSA,
			Hash:                      config.Hash(),
			PreferredHash:             []uint8{8}, // SHA-256
			FlagsValid:                true,
			FlagEncryptStorage:        true,
			FlagEncryptCommunications: true,
			IssuerKeyId:               &e.PrimaryKey.KeyId,
			KeyLifetimeSecs:           &keyLifetimeSecs,
		},
	}
	return &e
}

func encodePrivateKey(out io.Writer, key *rsa.PrivateKey) {
	w, err := armor.Encode(out, openpgp.PrivateKeyType, make(map[string]string))
    if err != nil {
        log.Println("Error creating OpenPGP Armor: %s", err)
    }

	pgpKey := packet.NewRSAPrivateKey(time.Now(), key)
    pgpKey.Serialize(w)
    w.Close()
}

func decodePrivateKey(in io.Reader) *packet.PrivateKey {

	block, _ := armor.Decode(in)

	reader := packet.NewReader(block.Body)
	pkt, _ := reader.Next()

	key, _ := pkt.(*packet.PrivateKey)
	return key
}

func encodePublicKey(out io.Writer, key *rsa.PrivateKey) {
	w, err := armor.Encode(out, openpgp.PublicKeyType, make(map[string]string))
    if err != nil {
        log.Println("Error creating OpenPGP Armor: %s", err)
    }

	pgpKey := packet.NewRSAPublicKey(time.Now(), &key.PublicKey)
    pgpKey.Serialize(w)
    w.Close()
}

func decodePublicKey(in io.Reader) *packet.PublicKey {

	block, _ := armor.Decode(in)

	reader := packet.NewReader(block.Body)
	pkt, _ := reader.Next()

	key, _ := pkt.(*packet.PublicKey)
	return key
}

func decodeSignature(in io.Reader) *packet.Signature {

	block, err := armor.Decode(in)
    if err != nil {
        log.Println("Error decoding OpenPGP Armor: %s", err)
    }

	reader := packet.NewReader(block.Body)
	pkt, err := reader.Next()

	sig, _ := pkt.(*packet.Signature)
	return sig
}

func PublicKeySerialise() string {
    public_key := SEC_ENTITY.PrimaryKey

    buf := new(bytes.Buffer)
    public_key.SerializeSignaturePrefix(buf)

    log.Println("Sending public key", buf.String())
    return buf.String()
}

func InitKeys(directory string, peer_id string) {
    key, _ := rsa.GenerateKey(rand.Reader, 4096)
    PRIVATE_KEY_FILE = filepath.Join(directory, peer_id+".privkey")
    private_key_file, _ := os.Create(PRIVATE_KEY_FILE)
    defer private_key_file.Close()

    PUBLIC_KEY_FILE = filepath.Join(directory, peer_id+".publickey")
    public_key_file, _ := os.Create(PUBLIC_KEY_FILE)
    defer public_key_file.Close()

	encodePrivateKey(private_key_file, key)
	encodePublicKey(public_key_file, key)

    // Create entity

	// open ascii armored public key
	pub_in, _ := os.Open(PUBLIC_KEY_FILE)
	defer pub_in.Close()
	pubKey := decodePublicKey(pub_in)

	// open ascii armored private key
	pri_in, _ := os.Open(PRIVATE_KEY_FILE)
	defer pri_in.Close()
	privKey := decodePrivateKey(pri_in)

	SEC_ENTITY = createEntityFromKeys(pubKey, privKey)

    // Read Public key
    readPublicKey()

    TRUSTED_PUBLIC_KEYS = make(map[string]string)
}

func readPublicKey() {
	in, _ := os.Open(PUBLIC_KEY_FILE)
	defer in.Close()
    chunk := make([]byte, 800)
    in.Read(chunk)
    PUBLIC_KEY_TEXT = string(chunk)
    log.Println("Sending public key", string(chunk))
}

func encodedProto(packet *message.GossipPacket) io.Reader{
    var byteData []byte
    if packet.Rumor!= nil {
        byteData, _ = protobuf.Encode(packet.Rumor)
    } else if packet.Status != nil {
        byteData, _ = protobuf.Encode(packet.Status)
    } else if packet.Private != nil {
        byteData, _ = protobuf.Encode(packet.Private)
    } else if packet.Request != nil {
        byteData, _ = protobuf.Encode(packet.Request)
    } else if packet.Reply != nil {
        byteData, _ = protobuf.Encode(packet.Reply)
    } else if packet.SRequest !=nil {
        byteData, _ = protobuf.Encode(packet.SRequest)
    } else if packet.SReply != nil {
        byteData, _ = protobuf.Encode(packet.SReply)
    }
    r := bytes.NewReader(byteData)
    return r
}

func SignPacket(packet *message.GossipPacket) string {
    r := encodedProto(packet)
    buf := new(bytes.Buffer)

    err := openpgp.ArmoredDetachSign(buf, SEC_ENTITY, r, nil)
    if err != nil {
        log.Println("Error signing %s", err)
    }

    return buf.String()

}

func VerifyPacket(packet *message.GossipPacket) bool {
    sign := packet.Signature.Sign
    peer := packet.Signature.By

    pub_key, public_key_exists := TRUSTED_PUBLIC_KEYS[peer]
    if !public_key_exists {
        return false
    }

    peer_pub_in := strings.NewReader(pub_key)
	pubKey := decodePublicKey(peer_pub_in)

    peer_sign := strings.NewReader(sign)
	sig := decodeSignature(peer_sign)

	hash := sig.Hash.New()
    r := encodedProto(packet)
	io.Copy(hash, r)

	err := pubKey.VerifySignature(hash, sig)
    return err==nil
}

func AddorUpdatePublicKey(peer string, public_key string) {
    old_public_key, public_key_exists := TRUSTED_PUBLIC_KEYS[peer]
    if !public_key_exists || old_public_key != public_key {
        TRUSTED_PUBLIC_KEYS[peer] = public_key
    }
    log.Println("Trusted public keys are", TRUSTED_PUBLIC_KEYS)
}

