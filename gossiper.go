package main

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "flag"
    "io/ioutil"
    "log"
    "math/rand"
    "net"
    "os"
    "os/signal"
    "path/filepath"
    "regexp"
    "strings"
    "strconv"
    "syscall"
    "time"
	"math"

    "github.com/dedis/protobuf"
    "github.com/Swarali/Peerster/message"
    "github.com/Swarali/Peerster/webserver"
    "github.com/Swarali/Peerster/security"
	"github.com/Swarali/Peerster/streaming"
)

// go run gossiper.go -UIPort=10000 -gossipAddr=127.0.0.1:5000 -name=nodeA -peers=127.0.0.1:5001_10.1.1.7:5002
var DEBUG bool
var PACKET_SIZE int
var RTIMER int
var NOFORWARD bool
var HOPLIMIT uint32
var UI_PORT string
var GOSSIP_PORT string
var CHUNK_SIZE int64
var STREAMING_SIZE int64
var ENCODING bool
var SHARING_DIRECTORY string
var BUDGET_THRESHOLD uint64
var MATCH_THRESHOLD int
var MessageQueue chan message.Message
var WebServerReceiveChannel chan message.ClientMessage
var WebServerSendChannel chan message.ClientMessage

func ListenOn(address string) *net.UDPConn {
    udpAddr, _ := net.ResolveUDPAddr("udp", address)
    udpConn, _ := net.ListenUDP("udp", udpAddr)
    return udpConn
}

type AckWait struct {
    Addr string
    Ack_id string
    Ack_wait chan bool
    Close bool}

type ReplyWait struct {
    FileName string
    Hash []byte
    Reply_wait chan *message.DataReply
    Close bool}

type ChunkData struct {
    Hash []byte
    Sources map[string]bool
}

type UploadedFile struct {
    FileSize int64
    MetaFileName string
    MetaHash []byte
    CData []*ChunkData
    MetaChunk []*MetaData
}

type MetaData struct {
	Hash []byte
	FileName string
}

type SRequestReceive struct {
    SRequest *message.SearchRequest
    IsDuplicate chan bool
}

type SReplyReceive struct {
    Origin string
    SReply *message.SearchResult
}

type SReplyWait struct {
    Key_words []string
    SReply_wait chan SReplyReceive
    Close bool}

type Gossiper struct {
    Name string
    UIConn *net.UDPConn
    GossipConn *net.UDPConn
    RumorChannel chan message.GossipMessage
    StatusChannel chan message.GossipMessage
    PrivateChannel chan message.GossipMessage
    RequestChannel chan message.GossipMessage
    ReplyChannel chan message.GossipMessage
    SRequestChannel chan message.GossipMessage
    SReplyChannel chan message.GossipMessage
    VectorTable map[string]uint32
    NextRoutingTable map[string]string
    NextRoutingSeq map[string]uint32
    RumorMessages map[string][]message.RumorMessage
    PrivateMessages map[string][]string
    UploadedFiles map[string]*UploadedFile
    PeerList map[string]bool
    TrustedPeerList map[string]bool
    AckReceiveChannel chan string
    AckSendChannel chan AckWait
    ReplyWaitChannel chan ReplyWait
    ReplyReceiveChannel chan *message.DataReply
    SRequestReceiveChannel chan SRequestReceive
    SRequestWaitChannel chan *message.SearchRequest
    SReplyWaitChannel chan SReplyWait
    SReplyReceiveChannel chan SReplyReceive
}

func NewGossiper(name, webport string,
                 peer_list []string) *Gossiper {
    peer_list_map := make(map[string]bool)
    trusted_peer_list_map := make(map[string]bool)
    for _, peer := range peer_list {
        peer_list_map[peer] = true
        trusted_peer_list_map[peer] = true
    }
    client_conn := ListenOn("127.0.0.1:" + UI_PORT)
    gossip_conn := ListenOn(GOSSIP_PORT)

    //vectorTable := map[string]int{name:1}
    gossiper:= &Gossiper{Name: name,
                          UIConn: client_conn,
                          GossipConn: gossip_conn,
                          RumorChannel: make(chan message.GossipMessage),
                          StatusChannel: make(chan message.GossipMessage),
                          PrivateChannel: make(chan message.GossipMessage),
                          RequestChannel: make(chan message.GossipMessage),
                          ReplyChannel: make(chan message.GossipMessage),
                          SRequestChannel: make(chan message.GossipMessage),
                          SReplyChannel: make(chan message.GossipMessage),
                          VectorTable: make(map[string]uint32),
                          NextRoutingTable: make(map[string]string),
                          NextRoutingSeq: make(map[string]uint32),
                          RumorMessages: make(map[string][]message.RumorMessage),
                          PrivateMessages: make(map[string][]string),
                          UploadedFiles: make(map[string]*UploadedFile),
                          PeerList: peer_list_map,
                          TrustedPeerList: trusted_peer_list_map,
                          AckReceiveChannel: make(chan string),
                          AckSendChannel: make(chan AckWait),
                          ReplyWaitChannel: make(chan ReplyWait),
                          ReplyReceiveChannel: make(chan *message.DataReply),
                          SRequestReceiveChannel: make(chan SRequestReceive),
                          SRequestWaitChannel: make(chan *message.SearchRequest),
                          SReplyWaitChannel: make(chan SReplyWait),
                          SReplyReceiveChannel: make(chan SReplyReceive),
                        }
    //Add self into the VectorTable
    gossiper.VectorTable[name] = 1
    log.Println("Start receiving client msg on", UI_PORT, ":", client_conn)
    go ReceiveClientMessage(client_conn)

    log.Println("Start receiving gossip msg on", GOSSIP_PORT, ":", gossip_conn)
    go ReceiveGossipMessage(gossip_conn)

    log.Println("Start receiving webclient msg on", webport)
    go ReceiveWebClientMessage(name, peer_list, webport)

    go gossiper.ProcessMessageQueue()
    return gossiper
}

func ReceiveClientMessage(conn *net.UDPConn) {
    for {
        packet := message.ClientMessage{}
        packetBytes := make([]byte, PACKET_SIZE)
        rlen, _, err := conn.ReadFrom(packetBytes)
        if err != nil {
            log.Println(err)
            continue
        }
        if rlen > PACKET_SIZE {
            log.Println("Size error: ", rlen)
            continue
        }
        protobuf.Decode(packetBytes, &packet)
        log.Println("Received message on", packet)
        MessageQueue<-message.Message{ClientMsg: &packet}
    }
}

func ReceiveGossipMessage(conn *net.UDPConn) {
    for {
        packet := &message.GossipPacket{}
        packetBytes := make([]byte, PACKET_SIZE)
        rlen, addr, err := conn.ReadFrom(packetBytes)
        if err != nil {
            log.Println(err)
            continue
        }
        if rlen > PACKET_SIZE {
            log.Println("Size error: ", rlen)
            continue
        }
        //fmt.Println("Packet received", packetBytes)
        protobuf.Decode(packetBytes, packet)
        relay_addr := addr.String()
        if relay_addr == GOSSIP_PORT {
            log.Println("Drop packets from the same node")
            continue
        }
        gossip_msg := &message.GossipMessage{Packet:*packet,
                                            Relay_addr:relay_addr}

        // Allow messages for processing only if the message has been signed from the trusted public keys.
        signature_message := packet.Signature
        if signature_message == nil {
            // This should never happen. Every message should be signed
            log.Println("No signature found for packet: ", packet)
            continue
        } else {
            // Process only those packets that are verified or that are route rumor message for public keys.
            verified := security.VerifyPacket(packet)
            is_route_rmr_msg := false
            if packet.Rumor != nil && packet.Rumor.Text == "" && packet.Rumor.PublicKey != "" {
                is_route_rmr_msg = true
            }
            if !verified && !is_route_rmr_msg {
                log.Println("Dropping packet since it is not verifiable", packet)
                continue
            }
        }

        MessageQueue<-message.Message{GossipMsg: gossip_msg}
    }
}

func ReceiveWebClientMessage(name string, peer_list []string, webport string) {
    webserver.WebServerReceiveChannel = WebServerReceiveChannel
    webserver.WebServerSendChannel = WebServerSendChannel
    go webserver.StartWebServer(name, peer_list, webport, SHARING_DIRECTORY)
    for message_from_webclient := range WebServerReceiveChannel {
        MessageQueue<-message.Message{ClientMsg:&message_from_webclient}
    }
}
func (gossiper *Gossiper) ProcessMessageQueue() {
    for msg := range MessageQueue {
        var gossip_packet message.GossipPacket
        var channel chan message.GossipMessage
        var relay_addr string
        if msg.GossipMsg != nil {
            gossip_packet = msg.GossipMsg.Packet
            relay_addr = msg.GossipMsg.Relay_addr
            if gossip_packet.Rumor != nil {
                channel = gossiper.RumorChannel
            //    if gossip_packet.Rumor.LastIP != nil && gossip_packet.Rumor.LastPort != nil {
            //    gossiper.UpdatePeer(gossip_packet.Rumor.LastIP.String()+":"+strconv.Itoa(*gossip_packet.Rumor.LastPort))
            //}
            } else if gossip_packet.Status != nil {
                channel = gossiper.StatusChannel
            } else if gossip_packet.Private != nil {
                gossip_packet.Private.HopLimit-=1
                channel = gossiper.PrivateChannel
            } else if gossip_packet.Request != nil {
                gossip_packet.Request.HopLimit-=1
                channel = gossiper.RequestChannel
            } else if gossip_packet.Reply != nil {
                gossip_packet.Reply.HopLimit-=1
                channel = gossiper.ReplyChannel
            } else if gossip_packet.SRequest != nil {
                gossip_packet.SRequest.Budget -=1
                channel = gossiper.SRequestChannel
            } else if gossip_packet.SReply != nil {
                gossip_packet.SReply.HopLimit -=1
                channel = gossiper.SReplyChannel
            }
            // Do not update peerlist for Peers directly connected to them.
            gossiper.UpdatePeer(relay_addr)
        } else {
            operation := msg.ClientMsg.Operation
            client_msg := msg.ClientMsg.Message
            if operation == "NewID" {
                gossiper.UpdateID(client_msg)
                continue
            } else if operation == "NewPeer" {
                gossiper.UpdatePeer(client_msg)
                continue
            } else if operation == "NewTrustedPeer" {
                gossiper.UpdateTrustedPeer(client_msg)
                continue
            } else if operation == "NewFileUpload" {
                go gossiper.UploadFile(client_msg)
                continue
            } else if operation == "NewFileDownload" {
                file_hash := msg.ClientMsg.HashValue
                destination := msg.ClientMsg.Destination
                go gossiper.DownloadFile(client_msg, destination, file_hash)
                continue
            } else if operation == "NewFileSearch" {
                key_words := strings.Split(client_msg, ",")
                budget := msg.ClientMsg.Budget
                go gossiper.SearchFiles(budget, key_words)
                continue
            } else if operation == "NewMessage" {
                gossip_packet = message.GossipPacket{}
                gossip_packet.Rumor = &message.RumorMessage{}
                gossip_packet.Rumor.Origin = gossiper.Name
                gossip_packet.Rumor.Text = client_msg
                relay_addr = "N/A"
                channel = gossiper.RumorChannel
            } else if operation == "NewPrivateMessage" {
                destination := msg.ClientMsg.Destination
                gossip_packet = message.GossipPacket{}
                gossip_packet.Private = &message.PrivateMessage{}
                gossip_packet.Private.Origin = gossiper.Name
                gossip_packet.Private.ID = 0
                gossip_packet.Private.Text = security.EncryptMessage(client_msg, destination)
                gossip_packet.Private.Destination = destination
                gossip_packet.Private.HopLimit = HOPLIMIT
                relay_addr = "N/A"
                channel = gossiper.PrivateChannel
            }
        }
        gossiper.PrintPacket(gossip_packet, relay_addr)
        channel<-message.GossipMessage{Packet:gossip_packet,
                                       Relay_addr:relay_addr}
    }
}

func (gossiper *Gossiper)PrintPacket(packet message.GossipPacket, relay_addr string) {
    if packet.Rumor != nil {
        // Do not print the Rumor if Text is empty.
        if packet.Rumor.Text == "" { return }
        if relay_addr == "N/A" {
            fmt.Printf("CLIENT %s %s\n", packet.Rumor.Text,
                       gossiper.Name)
            log.Printf("CLIENT %s %s\n", packet.Rumor.Text,
                       gossiper.Name)
        } else {
            fmt.Printf("RUMOR origin %s from %s ID %d contents %s\n", packet.Rumor.Origin, relay_addr, packet.Rumor.ID,
                            packet.Rumor.Text)
            log.Printf("RUMOR origin %s from %s ID %d contents %s\n", packet.Rumor.Origin, relay_addr, packet.Rumor.ID,
                            packet.Rumor.Text)
        }
    } else if packet.Status != nil {
        status_str := fmt.Sprintf("STATUS from %s", relay_addr)
        for _, peer_status := range packet.Status.Want {
            peer_status_str := fmt.Sprintf("origin %s nextID %d", peer_status.Identifier, peer_status.NextID)
            status_str = fmt.Sprintf("%s %s", status_str, peer_status_str)
        }
        fmt.Println(status_str)
        log.Println(status_str)
    } else if packet.Private != nil {
        private_msg := packet.Private
        if private_msg.Destination == gossiper.Name {
            fmt.Printf("PRIVATE: %s:%d:%s\n", private_msg.Origin,
                       private_msg.HopLimit, private_msg.Text)
            log.Printf("PRIVATE: %s:%d:%s\n", private_msg.Origin,
                       private_msg.HopLimit, private_msg.Text)
        } else { return }
    }
    gossiper.PrintPeers()
}

func (gossiper *Gossiper)PrintPeers() {
    var peer_map map[string]bool
    peer_map = gossiper.PeerList
    peer_list := []string{}
    for peer, _ := range(peer_map) { peer_list = append(peer_list, peer) }
    fmt.Println(strings.Join(peer_list, ","))
    log.Println(strings.Join(peer_list, ","))
}

func (gossiper *Gossiper)UpdatePeer(relay_addr string) {
    if relay_addr == "N/A" { return }
    if relay_addr == GOSSIP_PORT { return }
    _, peer_exists := gossiper.PeerList[relay_addr]
    if peer_exists == false {
        log.Println("Updating Peer: ", relay_addr)
        gossiper.PeerList[relay_addr] = true
        WebServerSendChannel<-message.ClientMessage{Operation:"NewPeer", Message: relay_addr}
    }
}

func (gossiper *Gossiper)UpdateTrustedPeer(relay_addr string) {
    if relay_addr == "N/A" { return }
    if relay_addr == GOSSIP_PORT { return }
    _, peer_exists := gossiper.TrustedPeerList[relay_addr]
    if peer_exists == false {
        log.Println("Updating Trusted Peer: ", relay_addr)
        gossiper.TrustedPeerList[relay_addr] = true
        WebServerSendChannel<-message.ClientMessage{Operation:"NewTrustedPeer", Message: relay_addr}
    }
}

func (gossiper *Gossiper)UpdateID(new_id string) {
    gossiper.Name = new_id
    gossiper.VectorTable[new_id] = 1
    WebServerSendChannel<-message.ClientMessage{Operation:"NewID", Message: new_id}
    gossiper.RouteRumorInit()
}

func (gossiper *Gossiper) UploadFile(file_name string) {
	if ENCODING { file_name = streaming.EncodeFile(file_name) }

    file_path := filepath.Join(SHARING_DIRECTORY, file_name)
    os.Link(filepath.Join("files", file_name), file_path)
    file, err := os.Open(file_path)
    if err != nil {
        fmt.Println("Error opening file:", file_name, err)
        log.Println("Error opening file:", file_name, err)
    }
    defer file.Close()
    file_info, _ := file.Stat()
    file_size := file_info.Size()

    var o_metafile *os.File
    var metaChunk []*MetaData
    var o_metafile_name string
    var chunk_hash []byte

    metafile_name := "meta_"+file_name
    metafile_path := filepath.Join(SHARING_DIRECTORY, metafile_name)
    metafile, err := os.Create(metafile_path)
    if err != nil {
        fmt.Println("Unable to create metafile", metafile_path)
        log.Println("Unable to create metafile", metafile_path)
        return
    }
    defer metafile.Close()

    offset := int64(0)
    cdata := make([]*ChunkData, 0)
    chunk_no := 1
    metadata_hash := sha256.New()
    for {
        if offset >= file_size { break }
        file_chunk_size := CHUNK_SIZE
        if file_size-offset < CHUNK_SIZE {
            file_chunk_size = file_size-offset
        }

        chunk := make([]byte, file_chunk_size)
        file.Read(chunk)

        chunk_file_path := filepath.Join(SHARING_DIRECTORY, "chunk_"+strconv.Itoa(chunk_no)+"_"+file_name)
        chunk_file, ch_err := os.Create(chunk_file_path)
        if chunk_file != nil {
            _, ch_err = chunk_file.Write(chunk)
        }
        if ch_err != nil {
            fmt.Println("Unable to write/create chunk", chunk_file_path)
            log.Println("Unable to write/create chunk", chunk_file_path)
            break
        }
        chunk_file.Close()

        if math.Mod(float64(chunk_no - 1),250) == 0 {
        	o_metafile_name = fmt.Sprintf("meta_%v_%s", float64(chunk_no - 1) / 250.0, file_name)
			o_metafile_path := filepath.Join(SHARING_DIRECTORY, o_metafile_name)
			o_metafile, err = os.Create(o_metafile_path)

			if err != nil {
				fmt.Println("Unable to create metafile", o_metafile_path)
				log.Println("Unable to create metafile", o_metafile_path)
				return
			}

			defer o_metafile.Close()
		}

        //Calculate and append SHA of the chunk
        h := sha256.New()
        h.Write(chunk)
        chunk_hash = h.Sum(nil)
        metafile.Write(chunk_hash)
		o_metafile.Write(chunk_hash)
        metadata_hash.Write(chunk_hash)

		if math.Mod(float64(chunk_no - 1),250) == 249 {
			new_hash := make([]byte, 32)
			copy(new_hash, chunk_hash)
			o := sha256.New()
			o.Write(new_hash)
			new_hash = o.Sum(nil)
			metaChunk = append(metaChunk, &MetaData{ Hash: new_hash,
			          								 FileName: o_metafile_name })
		}

        chunk_data := &ChunkData{Hash: chunk_hash, Sources: make(map[string]bool)}
        chunk_data.Sources[gossiper.Name] = true
        cdata = append(cdata, chunk_data)
        log.Println("Chunk", cdata)

        offset +=file_chunk_size
        chunk_no+=1
    }

	new_hash := make([]byte, 32)
	copy(new_hash, chunk_hash)
	o := sha256.New()
	o.Write(new_hash)
	new_hash = o.Sum(nil)
	metaChunk = append(metaChunk, &MetaData{ Hash: new_hash,
											 FileName: o_metafile_name })

    //Calculate SHA of metafile
    metafile_hash := metadata_hash.Sum(nil)

	fmt.Printf("FINAL CODED: %x %v \n", metafile_hash, len(metafile_hash))

    gossiper.UploadedFiles[file_name] = &UploadedFile{FileSize: file_size,
                                        MetaFileName: metafile_name,
                                        MetaHash: metafile_hash,
                                        MetaChunk: metaChunk,
                                        CData: cdata}

	createSizeFile(file_name, file_size)

    log.Println("Uploaded and chunked file:", file_name, "into", chunk_no, ":", gossiper.UploadedFiles[file_name])
    WebServerSendChannel<-message.ClientMessage{Operation:"NewFileUpload", Message: file_name}
}

func createSizeFile(file_name string, size int64) {
	file_name = fmt.Sprintf("size_%s", file_name)
	file_path := filepath.Join(SHARING_DIRECTORY, file_name)
	file, err := os.Create(file_path)

	if err != nil {
		fmt.Println("Unable to create sizefile", file_path)
		log.Println("Unable to create sizefile", file_path)
		return
	}

	file.Write([]byte(strconv.Itoa(int(size))))

	defer file.Close()
}

func GetLastHash(reply *message.DataReply, file_name string, file_hash []byte, index int, f_metafile *os.File) ([]byte , string) {
	if file_name != reply.FileName || hex.EncodeToString(file_hash) != hex.EncodeToString(reply.HashValue) {
		fmt.Println("File Name or Hash Value does not match", file_name, reply.FileName)
		log.Println("File Name or Hash Value does not match", file_name, reply.FileName)
	}

	metafile_name := fmt.Sprintf("meta_%v_%s", index, file_name)
	metafile_path := filepath.Join(SHARING_DIRECTORY, metafile_name)
	metafile, err := os.Create(metafile_path)
	if err != nil {
		fmt.Println("Unable to create metafile", metafile_path)
		log.Println("Unable to create metafile", metafile_path)
		return nil, ""
	}
	metafile.Write(reply.Data)
	metafile.Close()

	f_metafile.Write(reply.Data)

	new_hash := make([]byte, 32)
	copy(new_hash, reply.Data[len(reply.Data) - 32 : len(reply.Data)])
	o := sha256.New()
	o.Write(new_hash)
	new_hash = o.Sum(nil)

	return new_hash, metafile_name
}

func (gossiper *Gossiper) DownloadMetadata(file_name string, destination string, file_hash []byte) {
    data_request := message.DataRequest{Origin: gossiper.Name,
                                        HopLimit: HOPLIMIT,
                                        FileName: file_name,
                                        HashValue: file_hash}
    //Send Request.
    fmt.Println("Downloading metafile of", file_name, "from", destination)
    log.Println("Downloading metafile of", file_name, "from", destination)
    possible_destinations:= []string{destination}

    //meta_reply := gossiper.GetReply(data_request, possible_destinations)
	//
    //total_seq := meta_reply.Seq

	metafile_name := "meta_" + file_name
	metafile_path := filepath.Join(SHARING_DIRECTORY, metafile_name)
	metafile, err := os.Create(metafile_path)
	if err != nil {
		fmt.Println("Unable to create metafile", metafile_path)
		log.Println("Unable to create metafile", metafile_path)
		return
	}

	var metaChunk []*MetaData
	var total_seq = 1
	var real_size int64

	for i := 0; i < total_seq; i++ {
		meta_reply := gossiper.GetReply(data_request, possible_destinations)
		total_seq = meta_reply.Seq
		real_size = meta_reply.Size

		last_hash, name := GetLastHash(meta_reply, file_name, file_hash, i, metafile)
		metaChunk = append(metaChunk, &MetaData{ Hash: last_hash, FileName: name })

		data_request = message.DataRequest{Origin: gossiper.Name,
			HopLimit: HOPLIMIT,
			FileName: file_name,
			HashValue: last_hash}
	}

	//Verify MetaParts
	metafile, _ = os.Open(metafile_path)
	metafile_info, _ := metafile.Stat()
	file_size := metafile_info.Size()
	data := make([]byte, file_size)
	metafile.Read(data)

    //chunk_no:= 0
    chunk_metadata := data
    CData := make([]*ChunkData, len(chunk_metadata)/32)
    //for {
    //    if len(chunk_metadata) <= 32*chunk_no { break }
    //    chunk_hash:= chunk_metadata[32*chunk_no: 32*chunk_no+32]
    //    chunk_data := &ChunkData{Hash: chunk_hash, Sources: make(map[string]bool)}
    //    CData[chunk_no] = chunk_data
    //    chunk_no+=1
    //}

    for chunk_no := 0; chunk_no < len(CData); chunk_no++ {
		chunk_hash := chunk_metadata[32 * chunk_no: 32 * (chunk_no + 1)]
		chunk_data := &ChunkData{Hash: chunk_hash, Sources: make(map[string]bool)}
		CData[chunk_no] = chunk_data
	}

	createSizeFile(file_name, real_size)

    gossiper.UploadedFiles[file_name] = &UploadedFile{FileSize: real_size,
                                        MetaFileName: metafile_name,
                                        MetaHash: file_hash,
                                        MetaChunk: metaChunk,
                                        CData: CData}

    log.Println("CData:", CData)
}

func (gossiper *Gossiper) DownloadFile(file_name string, destination string, file_hash []byte) {
    file_data, metadata_downloaded := gossiper.UploadedFiles[file_name]
    if !metadata_downloaded {
        if destination == "" {
            fmt.Println("Insuffucient information, destination missing for", file_name)
            log.Println("Insuffucient information, destination missing for", file_name)
        }
        if hex.EncodeToString(file_hash) == "" {
            fmt.Println("Insufficient information, file hash is missing for", file_name)
            log.Println("Insufficient information, file hash is missing for", file_name)
        }
        gossiper.DownloadMetadata(file_name, destination, file_hash)
        file_data, metadata_downloaded = gossiper.UploadedFiles[file_name]
        for _, chunk_data := range(file_data.CData) {
            chunk_data.Sources[destination] = true
        }
    }
    gossiper.DownloadChunks(file_name)
}

func (gossiper *Gossiper) DownloadChunks(file_name string) {
    chunk_data := gossiper.UploadedFiles[file_name].CData
    file_size := int64(0)
    file_path := filepath.Join(SHARING_DIRECTORY, file_name)
    file, err := os.Create(file_path)
    if err != nil {
        fmt.Println("Unable to create file", file_path)
        log.Println("Unable to create file", file_path)
        return
    }
    defer file.Close()

    for chunk_no, chunk := range(chunk_data) {
        chunk_hash_req := chunk.Hash
        chunk_request := message.DataRequest{Origin: gossiper.Name,
                                            HopLimit: HOPLIMIT,
                                            FileName: file_name,
                                            HashValue: chunk_hash_req}
        //Send Request.
        possible_destinations:= make([]string, 0)
        for dest := range chunk.Sources {
        	possible_destinations = append(possible_destinations, dest)
		}
        reply := gossiper.GetReply(chunk_request, possible_destinations)
        fmt.Println("Downloading", file_name, "chunk", chunk_no+1, "from", reply.Origin)
        log.Println("Downloading", file_name, "chunk", chunk_no+1, "from", reply.Origin)
        if file_name != reply.FileName || hex.EncodeToString(chunk_hash_req) != hex.EncodeToString(reply.HashValue) {
            fmt.Println("File Name or Hash Value does not match for", file_name)
            log.Println("File Name or Hash Value does not match for", file_name)
            continue
        }
        chunk_hash := sha256.New()
        chunk_hash.Write(reply.Data)
        if hex.EncodeToString(chunk_hash_req) != hex.EncodeToString(chunk_hash.Sum(nil)) {
            fmt.Println("Hash of the data does not match Hash Value for", file_name)
            log.Println("Hash of the data does not match Hash Value for", file_name)
            continue
        }
        chunk_path := filepath.Join(SHARING_DIRECTORY, "chunk_"+strconv.Itoa(chunk_no+1)+"_"+file_name)
        chunkfile, err := os.Create(chunk_path)
        if err != nil {
            fmt.Println("Unable to create chunkfile", chunk_path)
            log.Println("Unable to create chunkfile", chunk_path)
            return
        }
        chunkfile.Write(reply.Data)
        chunkfile.Close()

        file.Write(reply.Data)
        file_size+=int64(len(reply.Data))
        gossiper.UploadedFiles[file_name].FileSize = file_size
    }
    createSizeFile(file_name, file_size)

    fmt.Println("RECONSTRUCTED file", file_name)
    log.Println("RECONSTRUCTED file", file_name)
    log.Println(gossiper.UploadedFiles)
    WebServerSendChannel<-message.ClientMessage{Operation:"NewFileUpload", Message: file_name}
}

func (gossiper *Gossiper)SearchFiles(budget int, key_words []string) {
    increment_budget := false
    if budget == 0 {
        increment_budget = true
        budget = 2
    }
    temp_stored_searches := make(map[string]map[uint64]map[string]bool)
    srequest := &message.SearchRequest{Origin: gossiper.Name, Budget: uint64(budget), Keywords: key_words}
    //gossiper.SRequestChannel<-message.GossipMessage{Packet: message.GossipPacket{SRequest: srequest}, Relay_addr: "N/A"}
    gossiper.ForwardSearchReq(srequest, "N/A")
    timer := time.NewTimer(5*time.Second)
    for {
        sreply_wait := make(chan SReplyReceive, 1)
        reply_wait_obj := SReplyWait{ Key_words: key_words,
                                      SReply_wait: sreply_wait}
        gossiper.SReplyWaitChannel <- reply_wait_obj
        select {
            case sreply := <-sreply_wait:
                origin := sreply.Origin
                reply := sreply.SReply
                file_name := reply.FileName
                meta_hash := reply.MetafileHash
                log.Println("SReply received", reply)
                chunk_list := make([]string, len(reply.ChunkMap))
                for i, chunk_no := range(reply.ChunkMap) { chunk_list[i] = strconv.Itoa(int(chunk_no)) }
                fmt.Printf("Found match %s at %s budget=%d metafile=%s chunks=%s\n", file_name, origin, budget, hex.EncodeToString(meta_hash), strings.Join(chunk_list, ","))
                log.Printf("Found match %s at %s budget=%d metafile=%s chunks=%s\n", file_name, origin, budget, hex.EncodeToString(meta_hash), strings.Join(chunk_list, ","))
                _, metadata_download_triggered := temp_stored_searches[file_name]
                if !metadata_download_triggered {
                    //Download metafile
                    go gossiper.DownloadMetadata(file_name, origin, meta_hash)
                    temp_stored_searches[file_name] = make(map[uint64]map[string]bool)
                }
                for _, chunk_no := range(reply.ChunkMap) {
                    _, is_new_chunk := temp_stored_searches[file_name][chunk_no]
                    if !is_new_chunk { temp_stored_searches[file_name][chunk_no] = make(map[string]bool) }
                    temp_stored_searches[file_name][chunk_no][origin] = true
                }

            case <-timer.C:
                log.Println("Timeout waiting for sreply", key_words, srequest.Budget)
                srequest.Budget = srequest.Budget*2
                if srequest.Budget <= BUDGET_THRESHOLD && increment_budget {
                    //srequest = &message.SearchRequest{Origin: gossiper.Name, Budget: uint64(budget), Keywords: key_words}
                    //gossiper.SRequestChannel<-message.GossipMessage{Packet: message.GossipPacket{SRequest: srequest}, Relay_addr: "N/A"}
                    gossiper.ForwardSearchReq(srequest, "N/A")
                    timer = time.NewTimer(5*time.Second)
                } else {
                    log.Println("Budget exceeded")
                }
        }
        reply_wait_obj.Close = true
        gossiper.SReplyWaitChannel<-reply_wait_obj
        close(sreply_wait)
        if srequest.Budget > BUDGET_THRESHOLD || !increment_budget {
            break
        }
        matches := 0
        for file_name, chunks := range(temp_stored_searches) {
            file_data, metadata_downloaded := gossiper.UploadedFiles[file_name]
            log.Println("Search InProgress", file_data, metadata_downloaded)
            if !metadata_downloaded { continue }
            if len(file_data.CData) == len(chunks) {
                matches+=1
            }
        }

        if matches >= 2 {
            break
        }
        log.Println("Search result", temp_stored_searches)
    }

    for file_name, chunks := range(temp_stored_searches) {
        file_data, metadata_downloaded:= gossiper.UploadedFiles[file_name]
        log.Println("Search Finished", file_data, metadata_downloaded)
        if !metadata_downloaded { continue }
        if len(file_data.CData) == len(chunks) {
            WebServerSendChannel<-message.ClientMessage{Operation:"NewFileReply", Message: file_name}
        }
        for chunk_no, chunk_data := range file_data.CData {
            chunk_data.Sources = chunks[uint64(chunk_no+1)]
        }
    }
    fmt.Println("SEARCH FINISHED")
    log.Println("SEARCH FINISHED")

}

func (gossiper *Gossiper) ForwardSearchReq(packet *message.SearchRequest, relay_addr string) {
    origin:= packet.Origin
    budget:=packet.Budget
    key_words:=packet.Keywords
    var peer_list []string
    for peer, _ := range gossiper.PeerList {
        if relay_addr != peer {
            peer_list = append(peer_list, peer)
        }
    }
    if len(peer_list) == 0 { return }
    peer_budget := int(int(budget)/len(peer_list))
    extra_budget := int(budget) - int(peer_budget)
    for _, peer := range(peer_list) {
        srequest_budget := peer_budget
        if extra_budget > 0 {
            srequest_budget+=1
            extra_budget-=1
        }
        if srequest_budget <= 0 { break }
        srequest := &message.SearchRequest{Origin:origin, Budget: uint64(srequest_budget), Keywords: key_words}
        gossiper.SendSearchRequest(srequest, peer)
    }
}

func (gossiper *Gossiper) GetReply(chunk_request message.DataRequest, possible_destinations []string) *message.DataReply {
    var reply *message.DataReply
    file_hash := chunk_request.HashValue
    file_name := chunk_request.FileName
    reply_wait := make(chan *message.DataReply, 1)
    reply_wait_obj := ReplyWait{FileName: file_name,
                                Hash: file_hash,
                                Reply_wait: reply_wait}
    gossiper.ReplyWaitChannel <- reply_wait_obj
    for {
        log.Println("Possible destinations:", possible_destinations)
        destination := possible_destinations[rand.Intn(len(possible_destinations))]
        chunk_request.Destination = destination
        gossiper.RequestChannel<-message.GossipMessage{Packet: message.GossipPacket{Request: &chunk_request}, Relay_addr: "N/A"}
        timer := time.NewTimer(60*time.Second)
        select {
            // Wait for reply or timeout
            case reply = <-reply_wait:
                log.Println("Reply received", reply)
                // De-register the object from the wait.
                reply_wait_obj.Close = true
                gossiper.ReplyWaitChannel<-reply_wait_obj
                close(reply_wait)
                return reply
            case <-timer.C:
                log.Println("Timeout waiting for hash", hex.EncodeToString(file_hash))
        }
    }
}

func (gossiper *Gossiper) ServeSearchRequest(srequest *message.SearchRequest) {
    timer := time.NewTimer(time.Second)
    matched_files := make(map[string]bool)
    for file_name, _:= range(gossiper.UploadedFiles) {
        for _, keyword:= range(srequest.Keywords) {
            matched, err:=regexp.MatchString(keyword, file_name)
            if err != nil {
                fmt.Println("Error while matching regex", err)
                log.Println("Error while matching regex", err)
            }
            if matched {
                matched_files[file_name] = true
            }
        }
    }
    if len(matched_files) > 0 {
        sreply := message.SearchReply{ Origin: gossiper.Name,
                                       Destination: srequest.Origin,
                                       HopLimit: HOPLIMIT}
        for file_name, _ := range(matched_files) {
            log.Println("Found match", file_name, srequest)
            file_data := gossiper.UploadedFiles[file_name]
            sresult := &message.SearchResult{FileName: file_name}
            sresult.MetafileHash = file_data.MetaHash
            for chunk_no, chunk_data := range(file_data.CData) {
                chunk_present := chunk_data.Sources[gossiper.Name]
                if chunk_present {
                    sresult.ChunkMap = append(sresult.ChunkMap, uint64(chunk_no+1))
                }
            }
            sreply.Results = append(sreply.Results, sresult)
        }
        gossiper.SReplyChannel<-message.GossipMessage{Packet: message.GossipPacket{SReply: &sreply}, Relay_addr: "N/A"}
    } else {
        log.Println("No match found", srequest, gossiper.UploadedFiles)
    }
    <-timer.C
    gossiper.SRequestWaitChannel<-srequest
}

func (gossiper *Gossiper) SendStatus() {
    for channel_packet := range gossiper.StatusChannel{
        relay_addr := channel_packet.Relay_addr
        packet := channel_packet.Packet.Status

        //Acknowledge the ack
        gossiper.AckReceiveChannel <- relay_addr
        peer_vector_map := make(map[string]uint32)
        for _, peer_status := range packet.Want {
            peer_vector_map[peer_status.Identifier] = peer_status.NextID
        }

        gossiper_has_new_msg := false
        var rumor_msg *message.RumorMessage
        for origin, next_id := range gossiper.VectorTable {
            peer_next_id, ok := peer_vector_map[origin]
            if ok != true { peer_next_id = 1 }
            if next_id > peer_next_id {
                gossiper_has_new_msg = true
                rumor_msg = &gossiper.RumorMessages[origin][peer_next_id-1]
                break
            }
        }
        if gossiper_has_new_msg {
            if NOFORWARD {
                log.Println("Not forwarding messages since noforward flag is set")
            } else {
                fmt.Println("MONGERING TEXT to "+relay_addr)
                log.Println("MONGERING TEXT to "+relay_addr)
                gossiper.PrintPeers()
                go gossiper.SendRumor(rumor_msg, relay_addr)
                //continue
            }
        }

        remote_has_new_msg := false
        for origin, peer_next_id := range peer_vector_map {
            next_id, ok := gossiper.VectorTable[origin]
            if ok != true { next_id = 1 }
            if next_id < peer_next_id {
                remote_has_new_msg = true
                break
            }
        }
        if remote_has_new_msg {
            go gossiper.SendAck(relay_addr)
        }
        if !gossiper_has_new_msg && !remote_has_new_msg {
            fmt.Println("IN SYNC WITH "+relay_addr)
            log.Println("IN SYNC WITH "+relay_addr)
            gossiper.PrintPeers()
        }
    }
}

func (gossiper *Gossiper) GossipMessages() {
    for channel_packet := range gossiper.RumorChannel {

        relay_addr := channel_packet.Relay_addr
        packet  := channel_packet.Packet.Rumor
        if relay_addr == "N/A" {
            next_id, ok := gossiper.VectorTable[packet.Origin]
            if !ok { next_id = 1 }
            packet.ID = next_id
            //packet.ID = gossiper.VectorTable[packet.Origin]
        } else {
            tmp := strings.Split(relay_addr, ":")
            last_ip := net.ParseIP(tmp[0])
            last_port, _ := strconv.Atoi(tmp[1])
            packet.LastIP = &last_ip
            packet.LastPort = &last_port
            log.Println("Packet:", packet.Origin, packet.ID, packet.Text, *packet.LastIP, *packet.LastPort)
        }
        // Check if the relay is a trusted relay
        _, relay_trusted := gossiper.TrustedPeerList[relay_addr]
        if packet.Text == "" && ! relay_trusted {
            // We send public key through route rumor messages only therefore no need to update public keys for other kind of rumor messages.
            log.Println("Peer", relay_addr, " is not in the trusted peer list")
            continue
        }

        next_id, ok := gossiper.VectorTable[packet.Origin]
        if !ok {
            //gossiper.RumorMessages[packet.Origin] = make(string[])
            gossiper.VectorTable[packet.Origin] = 1
            next_id = 1
        }

        gossiper.UpdateRoutingTable(channel_packet)
        if packet.Text == "" {
            security.AddorUpdatePublicKey(packet.Origin, packet.PublicKey)
            var peer_to_send string
            var peer_list []string
            last_relay := channel_packet.Relay_addr
            for peer, _ := range gossiper.PeerList {
                if peer != last_relay {
                peer_list = append(peer_list, peer) }
            }

            if len(peer_list) != 0 && packet.Origin != gossiper.Name {
                peer_to_send = peer_list[rand.Intn(len(peer_list))]
                log.Println("Send Route ", packet.Origin, *packet.LastIP, *packet.LastPort, "to ", peer_to_send)
                fmt.Println("MONGERING ROUTE to "+peer_to_send)
                log.Println("MONGERING ROUTE to "+peer_to_send)
                gossiper.SendRumor(packet, peer_to_send)
            }
        } else if packet.ID == next_id {
            // Update messages only if the message is non-empty.
            message_list := append(gossiper.RumorMessages[packet.Origin],
                                   *packet)
            gossiper.RumorMessages[packet.Origin] = message_list
            gossiper.VectorTable[packet.Origin] += 1
            // Notify the WebServer.
            WebServerSendChannel<-message.ClientMessage{Operation:"NewMessage", Message:packet.Text, Origin:packet.Origin}
            log.Println("RumorMessages:", gossiper.RumorMessages,
                        "Vector Table:", gossiper.VectorTable)

            if NOFORWARD { log.Println("Not forwarding messages since noforward flag is set")
            } else { go gossiper.TransmitMessage(channel_packet,
                                                     gossiper.PeerList)
            }
        } else {
            //Discard the message since it is not in order
            log.Println("Discarding the message", packet.ID,
                        packet.Origin, packet.Text)
            log.Println("RumorMessages:", gossiper.RumorMessages,
                        "Vector Table:", gossiper.VectorTable)
        }

        if relay_addr != "N/A" { go gossiper.SendAck(relay_addr)}
    }
}

func (gossiper *Gossiper) IndexMetaChunk(file_name string, hash []byte) int {
	for i := range gossiper.UploadedFiles[file_name].MetaChunk {
		if hex.EncodeToString(gossiper.UploadedFiles[file_name].MetaChunk[i].Hash) == hex.EncodeToString(hash) {
			return i
		}
	}

	return -1
}

func (gossiper *Gossiper) IsChunkPresent(req message.DataRequest, relay_addr string) bool{
    file_name := req.FileName
    hash_value := req.HashValue
    file_data, metadata_present := gossiper.UploadedFiles[file_name]
    if !metadata_present { return false }
    log.Println(file_data.CData)
    log.Println(hex.EncodeToString(hash_value))
    meta_hash := file_data.MetaHash

    // Metahash found
    if hex.EncodeToString(meta_hash) == hex.EncodeToString(hash_value) {
        log.Println("Metadata", hex.EncodeToString(hash_value), "is present")

        meta_no := -1

        if len(gossiper.UploadedFiles[file_name].MetaChunk) > 1 { meta_no = 0 }

        go gossiper.SendReply(req.Origin, file_name, 0, relay_addr, meta_no)
        return true
    }

	// MetaParts Found
    index := gossiper.IndexMetaChunk(file_name, hash_value)

    if  index != -1 && len(gossiper.UploadedFiles[file_name].MetaChunk) > 1 {
		log.Println("Metadata Part", index, hex.EncodeToString(hash_value), "is present")
    	go gossiper.SendReply(req.Origin, file_name, 0, relay_addr, index + 1)
    	return true
	}

    // Chunks found
    for chunk_no, chunk_data := range file_data.CData {
        chunk_hash := hex.EncodeToString(chunk_data.Hash)
        if chunk_hash == hex.EncodeToString(hash_value)  && len(chunk_data.Sources) > 0 {
            for source := range(chunk_data.Sources) {
                if source == gossiper.Name {
                    log.Println("Chunk", chunk_no+1, ":", hex.EncodeToString(hash_value), "is present")
                    go gossiper.SendReply(req.Origin, file_name, chunk_no+1, relay_addr, -1)
                    return true
                }
            }
        }
    }
    return false
}

func (gossiper *Gossiper) GossipReplyMessages() {
    for channel_packet := range gossiper.ReplyChannel {
        // relay_addr := channel_packet.Relay_addr
        packet  := channel_packet.Packet.Reply
        if packet.Destination == gossiper.Name {
            gossiper.ReplyReceiveChannel<-packet
            continue
        }
        if packet.HopLimit <=0 {
            // Discard the packet.
            log.Println("Discarding reply message due to hop limit exceeded:",
                        packet.Origin, packet.FileName,
                        packet.HopLimit,
                        packet.Destination)
            continue
        }
        next_addr, ok := gossiper.NextRoutingTable[packet.Destination]
        if !ok {
            // Discard the packet.
            log.Println("Discarding reply message due to no entry in routing table:",
                        packet.Origin, packet.FileName,
                        packet.HopLimit,
                        packet.Destination)
            continue
        }
        if NOFORWARD {
            fmt.Println("Not forwarding reply message")
            log.Println("Not forwarding reply message")
        } else { go gossiper.SendReplyMessage(packet, next_addr) }
    }
}

func (gossiper *Gossiper) GossipRequestMessages() {
    for channel_packet := range gossiper.RequestChannel {
        relay_addr := channel_packet.Relay_addr
        packet  := channel_packet.Packet.Request
        log.Println("Received request", packet)
        if gossiper.IsChunkPresent(*packet, relay_addr) {
            log.Println("Found the requested chunk")
            continue
        }
        if packet.Destination == gossiper.Name {
            log.Println("File", packet.FileName, "cannot be found on", gossiper.Name)
            continue
        }
        if packet.HopLimit <=0 {
            // Discard the packet.
            log.Println("Discarding request message due to hop limit exceeded:",
                        packet.Origin, packet.FileName,
                        packet.HopLimit,
                        packet.Destination)
            continue
        }
        next_addr, ok := gossiper.NextRoutingTable[packet.Destination]
        if !ok {
            // Discard the packet.
            log.Println("Discarding request message due to no entry in routing table:",
                        packet.Origin, packet.FileName,
                        packet.HopLimit,
                        packet.Destination)
            continue
        }
        if NOFORWARD {
            fmt.Println("Not forwarding request message")
            log.Println("Not forwarding request message")
        } else { go gossiper.SendRequestMessage(packet, next_addr) }
    }
}

func (gossiper *Gossiper) GossipSearchRequests() {
    for channel_packet := range gossiper.SRequestChannel {
        relay_addr := channel_packet.Relay_addr
        packet  := channel_packet.Packet.SRequest
        log.Println("Received srequest", packet)
        is_duplicate_chan := make(chan bool)
        gossiper.SRequestReceiveChannel<- SRequestReceive{ SRequest: packet, IsDuplicate: is_duplicate_chan}
        // Check if it is duplicate packet
        is_duplicate:= <-is_duplicate_chan
        if is_duplicate {
            log.Println("Discarding due to SRequest already being served")
            continue
        } else {
            log.Println("Not discarding SRequest")
        }
        gossiper.SRequestWaitChannel<-packet
        go gossiper.ServeSearchRequest(packet)
        // Search if packet is present
        if packet.Budget <=0 {
            // Discard the packet.
            log.Println("Discarding due to budget exceeded:",
                        packet.Origin, packet.Budget,
                        packet.Keywords)
            continue
        }
        if NOFORWARD {
            fmt.Println("Not forwarding srequest")
            log.Println("Not forwarding srequest")
        } else {
            go gossiper.ForwardSearchReq(packet, relay_addr)
        }
    }
}

func (gossiper *Gossiper) GossipSearchReply() {
    for channel_packet := range gossiper.SReplyChannel {
        // relay_addr := channel_packet.Relay_addr
        packet  := channel_packet.Packet.SReply
        log.Println("Received sreply", packet)
        if packet.Destination == gossiper.Name {
            for _, result := range(packet.Results) {
                sreply:= SReplyReceive{Origin:packet.Origin, SReply: result}
                gossiper.SReplyReceiveChannel<-sreply
            }
            continue
        }
        if packet.HopLimit <=0 {
            // Discard the packet.
            log.Println("Discarding search reply message due to hop limit exceeded:",
                        packet.Origin,
                        packet.HopLimit,
                        packet.Destination)
            continue
        }
        next_addr, ok := gossiper.NextRoutingTable[packet.Destination]
        if !ok {
            // Discard the packet.
            log.Println("Discarding search reply message due to no entry in routing table:",
                        packet.Origin,
                        packet.HopLimit,
                        packet.Destination)
            continue
        }
        if NOFORWARD {
            fmt.Println("Not forwarding reply message")
            log.Println("Not forwarding reply message")
        } else { go gossiper.SendSearchReply(packet, next_addr) }
    }
}

func (gossiper *Gossiper) GossipPrivateMessages() {
    for channel_packet := range gossiper.PrivateChannel {
        // relay_addr := channel_packet.Relay_addr
        packet  := channel_packet.Packet.Private
        if packet.Destination == gossiper.Name {
            msg := security.DecryptMessage(packet.Text)
            WebServerSendChannel<-message.ClientMessage{Operation:"NewPrivateMessage", Message:msg, Origin:packet.Origin}
            continue
        }
        if packet.Origin == gossiper.Name {
            //msg := security.DecryptMessage(packet.Text)
            WebServerSendChannel<-message.ClientMessage{Operation:"NewPrivateMessage", Message:packet.Text, Destination:packet.Destination}
        }
        if packet.HopLimit <=0 {
            // Discard the packet.
            log.Println("Discarding private message due to hop limit exceeded:",
                        packet.Origin, packet.Text,
                        packet.HopLimit,
                        packet.Destination)
            continue
        }
        next_addr, ok := gossiper.NextRoutingTable[packet.Destination]
        if !ok {
            // Discard the packet.
            log.Println("Discarding private message due to no entry in routing table:",
                        packet.Origin, packet.Text,
                        packet.HopLimit,
                        packet.Destination)
            continue
        }
        if NOFORWARD {
            fmt.Println("Not forwarding private message")
            log.Println("Not forwarding private message")
        } else { go gossiper.SendPrivateMessage(packet, next_addr) }
    }
}

func (gossiper *Gossiper) UpdateRoutingTable(channel_packet message.GossipMessage) {
    relay_addr := channel_packet.Relay_addr
    packet  := channel_packet.Packet.Rumor
    if packet.Origin == gossiper.Name || relay_addr == "N/A" { return }
    log.Println("Received Route rumor about ", packet.Origin,
                " from ", relay_addr)
    next_seq, ok := gossiper.NextRoutingSeq[packet.Origin]
    is_direct:=false
    if next_seq == packet.ID {
        if packet.LastIP == nil && packet.LastPort == nil && (!ok || (next_seq <= packet.ID)) {
            is_direct=true
        }
    }
    if !ok || (next_seq < packet.ID) || is_direct {
        if is_direct {
            fmt.Printf("DIRECT-ROUTE FOR %s: %s\n", packet.Origin, relay_addr)
            log.Printf("DIRECT-ROUTE FOR %s: %s\n", packet.Origin, relay_addr)
        }
        gossiper.NextRoutingSeq[packet.Origin] = packet.ID
        if relay_addr != "N/A" {
            _, ok := gossiper.NextRoutingTable[packet.Origin]
            gossiper.NextRoutingTable[packet.Origin] = relay_addr
            if !ok {
                WebServerSendChannel<-message.ClientMessage{Operation: "NewRoute", Message: packet.Origin}
            }
            fmt.Printf("DSDV %s: %s\n", packet.Origin, relay_addr)
            log.Printf("DSDV %s: %s\n", packet.Origin, relay_addr)
        }
    }
}

func (gossiper *Gossiper) TransmitMessage(channel_packet message.GossipMessage,
                                          peer_list_map map[string]bool) {
    log.Println("Sending message")
    last_relay := channel_packet.Relay_addr
    packet := channel_packet.Packet
    for peer, _ := range peer_list_map {
        if peer != last_relay {

            // Wait for ack or timeout
            timer := time.NewTimer(time.Second)
            ack_wait := make(chan bool, 1)
            ack_id := packet.Rumor.Origin + ":" + fmt.Sprint(packet.Rumor.ID)
            ack_wait_obj := AckWait{Addr: peer,
                                    Ack_id : ack_id,
                                    Ack_wait: ack_wait}
            gossiper.AckSendChannel<- ack_wait_obj
            go gossiper.SendRumor(packet.Rumor, peer)
            select {
                case <-ack_wait:
                    log.Println("Ack received")
                case <-timer.C:
                    log.Println("Timeout")
                    // De-register the object from the wait.
                    gossiper.AckSendChannel<-ack_wait_obj
            }
        }
    }

}
func (gossiper *Gossiper) TransmitMessageWithRumorMongering(channel_packet message.GossipMessage,
                                          peer_list_map map[string]bool) {
    log.Println("Sending message")
    var peer_list []string
    last_relay := channel_packet.Relay_addr
    packet := channel_packet.Packet
    for peer, _ := range peer_list_map {
        if peer != last_relay {
        peer_list = append(peer_list, peer) }
    }
    if len(peer_list) == 0 { return }

    // Pick up a random peer
    var peer_to_send string
    var flipped_coin bool
    for {
        // If only one peer known do not do this for loop
        for {
            peer_to_send = peer_list[rand.Intn(len(peer_list))]
        }

        if flipped_coin {
            fmt.Println("FLIPPED COIN sending rumor to "+peer_to_send)
            log.Println("FLIPPED COIN sending rumor to "+peer_to_send)
            gossiper.PrintPeers()
        } else {
            fmt.Println("MONGERING TEXT to "+peer_to_send)
            log.Println("MONGERING TEXT to "+peer_to_send)
            gossiper.PrintPeers()
        }

        // Wait for ack or timeout
        timer := time.NewTimer(time.Second)
        ack_wait := make(chan bool, 1)
        ack_id := packet.Rumor.Origin + ":" + fmt.Sprint(packet.Rumor.ID)
        ack_wait_obj := AckWait{Addr: peer_to_send,
                                Ack_id : ack_id,
                                Ack_wait: ack_wait}
        gossiper.AckSendChannel<- ack_wait_obj
        go gossiper.SendRumor(packet.Rumor, peer_to_send)
        select {
            case <-ack_wait:
                log.Println("Ack received")
            case <-timer.C:
                log.Println("Timeout")
                // De-register the object from the wait.
                gossiper.AckSendChannel<-ack_wait_obj
        }

        if (rand.Int() % 2) == 0 { break }
        flipped_coin = true
    }
}

func (gossiper *Gossiper)SendGossip(packet message.GossipPacket,
                                    peer_to_send string) {
    if packet.Signature == nil{
        sign := security.SignPacket(&packet)
        packet.Signature = &message.SignatureMessage{Sign: sign, By: gossiper.Name}
    }
    log.Println("Peer to send is ", peer_to_send,
                "Packet to send", packet)
    packetBytes, proto_err := protobuf.Encode(&packet)
    //fmt.Println("Packet sent", packetBytes)
    if proto_err != nil {
        fmt.Println("Error while sending packet:", packet, proto_err)
        log.Println("Error while sending packet:", packet, proto_err)
        os.Exit(1)
    }
    udpAddr, _ := net.ResolveUDPAddr("udp", peer_to_send)
    gossiper.GossipConn.WriteToUDP(packetBytes, udpAddr)
}

func (gossiper *Gossiper)SendRumor(rumor *message.RumorMessage, peer_to_send string) {
    log.Println("Send Rumor", rumor.Origin, rumor.ID,
                rumor.Text, rumor.LastIP, rumor.LastPort)
    gossiper.SendGossip(message.GossipPacket{ Rumor: rumor},
                        peer_to_send)
}

func (gossiper *Gossiper)SendAck(relay_addr string) {
    status_packet := message.StatusPacket{}
    for origin, val := range gossiper.VectorTable {
        peer_status := message.PeerStatus {Identifier: origin,
                                           NextID:val}
        status_packet.Want = append(status_packet.Want, peer_status)
    }
    // Send Ack
    log.Println("Send Ack", status_packet)
    gossiper.SendGossip(message.GossipPacket{Status: &status_packet},
                        relay_addr)
}

func (gossiper *Gossiper)SendPrivateMessage(private_msg *message.PrivateMessage, next_addr string) {
    log.Println("Send PM", private_msg)
    gossiper.SendGossip(message.GossipPacket{Private: private_msg}, next_addr)
}

func (gossiper *Gossiper)SendRequestMessage(req *message.DataRequest, next_addr string) {
    log.Println("Send Request", req)
    gossiper.SendGossip(message.GossipPacket{Request: req}, next_addr)
}

func (gossiper *Gossiper) SendReplyMessage(reply *message.DataReply, relay_addr string) {
    log.Println("Send Reply to ", relay_addr)
    gossiper.SendGossip(message.GossipPacket{Reply: reply}, relay_addr)
}

func (gossiper *Gossiper) SendReply(destination string, file_name string, chunk_no int, relay_addr string, meta_no int) {
    chunk_reply := message.DataReply{Origin: gossiper.Name, Destination:destination,
    FileName: file_name, HopLimit: HOPLIMIT, Seq: len(gossiper.UploadedFiles[file_name].MetaChunk),
    Size: gossiper.UploadedFiles[file_name].FileSize }

    var path string
    var hash []byte
    if chunk_no == 0 {
    	if meta_no == -1 {
			// Return metadata file
			path = filepath.Join(SHARING_DIRECTORY, "meta_" + file_name)
			hash = gossiper.UploadedFiles[file_name].MetaHash
		} else {
			// Return metadata part file
			path = filepath.Join(SHARING_DIRECTORY, fmt.Sprintf("meta_%v_%s", meta_no, file_name))
			if meta_no == 0 {
				hash = gossiper.UploadedFiles[file_name].MetaHash
			} else {
				hash = gossiper.UploadedFiles[file_name].MetaChunk[meta_no - 1].Hash
			}
		}
    } else {
        path =filepath.Join(SHARING_DIRECTORY, "chunk_"+strconv.Itoa(chunk_no)+"_"+file_name)
        hash = gossiper.UploadedFiles[file_name].CData[chunk_no-1].Hash
    }
    file, open_err := os.Open(path)
    file_info, stat_err := file.Stat()
    if open_err!= nil || stat_err != nil {
        fmt.Println("Error while accessing", path, open_err, stat_err)
        log.Println("Error while accessing", path, open_err, stat_err)
        return
    }
    file_size := file_info.Size()
    defer file.Close()
    data := make([]byte, file_size)
    file.Read(data)
    chunk_reply.Data = data
    chunk_reply.HashValue = hash

    log.Println("Replying to the request for", hash,
                "to", destination)
    gossiper.SendReplyMessage(&chunk_reply, relay_addr)
}

func (gossiper *Gossiper) SendSearchReply(reply *message.SearchReply, relay_addr string) {
    log.Println("Send Search reply to ", relay_addr)
    gossiper.SendGossip(message.GossipPacket{SReply: reply}, relay_addr)
}

func (gossiper *Gossiper) SendSearchRequest(search *message.SearchRequest, relay_addr string) {
    log.Println("Send Search to ", relay_addr)
    gossiper.SendGossip(message.GossipPacket{SRequest: search}, relay_addr)
}

func (gossiper *Gossiper) WaitForAck() {
    // Waiting for Ack
    wait_map := make(map[string]map[string]chan bool)
    for {
        select {
            // Some scope for logic improvement??
            case peer := <-gossiper.AckReceiveChannel:
                channels, ok := wait_map[peer]
                if ok {
                    for _, channel := range channels {
                        channel <- true
                    }
                    wait_map[peer] = nil
                }
            case msg := <-gossiper.AckSendChannel:
                addr := msg.Addr
                ack_id := msg.Ack_id
                ack_wait := msg.Ack_wait
                val, ok := wait_map[addr]
                if !ok || val == nil {
                    wait_map[addr] = make(map[string]chan bool)
                }
                // If timeout occurs we send back ack_wait to 
                // de-register it from wait_map.
                old_ack_chan, old_ack_ok := wait_map[addr][ack_id]
                if old_ack_ok {
                    delete(wait_map[addr], ack_id)
                    close(old_ack_chan)
                } else {
                    wait_map[addr][ack_id] = ack_wait
                }
        }
    }
}

func (gossiper *Gossiper) WaitForReply() {
    // Waiting for Reply
    wait_map := make(map[string] chan *message.DataReply)
    for {
        select {
            case reply := <-gossiper.ReplyReceiveChannel:
                file_name := reply.FileName
                hash_string := hex.EncodeToString(reply.HashValue)
                key := file_name+":"+hash_string
                channel, ok := wait_map[key]
                if ok {
                    channel <- reply
                    delete(wait_map, key)
                }
            case req := <-gossiper.ReplyWaitChannel:
                file_name := req.FileName
                hash_string := hex.EncodeToString(req.Hash)
                reply_wait := req.Reply_wait
                close_chan := req.Close
                key := file_name+":"+hash_string
                // If timeout occurs we send back req_wait to 
                // de-register it from wait_map.
                _, old_ack_ok := wait_map[key]
                if close_chan {
                    if old_ack_ok {
                        delete(wait_map, key)
                    }
                } else {
                    wait_map[key] = reply_wait
                }
        }
    }
}

func (gossiper *Gossiper) WaitForSRequest() {
    // Waiting for SRequests
    currently_serving := make([]*message.SearchRequest, 0)
    for {
        select {
            case srequest := <-gossiper.SRequestReceiveChannel:
                packet:= srequest.SRequest
                found:=false
                is_duplicate_chan := srequest.IsDuplicate
                for _, serv_pack := range(currently_serving) {
                    if serv_pack.Origin == packet.Origin && strings.Join(serv_pack.Keywords, ",") == strings.Join(packet.Keywords, ",") {
                        is_duplicate_chan<-true
                        found=true
                        break
                    }
                }
                if !found {
                    is_duplicate_chan<-false
                }
            case sreq := <-gossiper.SRequestWaitChannel:
                found:=-1
                for i, serv_pack := range(currently_serving) {
                    if serv_pack==sreq {
                        found=i
                        break
                    }
                }
                if found!= -1{
                    currently_serving = append(currently_serving[:found], currently_serving[found+1:]...)
                } else {
                    currently_serving = append(currently_serving, sreq)
                }
        }
    }
}

func (gossiper *Gossiper) WaitForSReply() {
    // Waiting for Reply
    wait_map := make(map[string] chan SReplyReceive)
    for {
        select {
            case reply := <-gossiper.SReplyReceiveChannel:
                channels_to_reply := make(map[chan SReplyReceive]bool)
                for kw, channel := range(wait_map) {
                    matched, _:=regexp.MatchString(kw, reply.SReply.FileName)
                    if matched {
                        channels_to_reply[channel] = true
                        delete(wait_map, kw)
                    }
                }

                for channel, _ := range(channels_to_reply) {
                    channel <- reply
                }
            case srep_wait := <-gossiper.SReplyWaitChannel:
                close_chan := srep_wait.Close
                if close_chan {
                    for _, kw := range(srep_wait.Key_words) {
                        _, is_key_present := wait_map[kw]
                        if is_key_present {
                            delete(wait_map, kw)
                        }
                    }
                } else {
                    for _, kw := range(srep_wait.Key_words) {
                        wait_map[kw] = srep_wait.SReply_wait
                    }
                }
        }
    }
}

func (gossiper *Gossiper) AntiEntropy(){
    for {
        timer := time.NewTimer(time.Second)
        <-timer.C
        var peer_map map[string]bool
        peer_map= gossiper.PeerList
        peer_list := []string{}
        if len(peer_list) == 0 { continue }
        for peer, _ := range(peer_map) { peer_list = append(peer_list, peer) }
        peer_to_send := peer_list[rand.Intn(len(peer_list))]
        gossiper.SendAck(peer_to_send)
    }
}

func (gossiper *Gossiper)SendRoute(origin, peer_to_send string) {
    log.Println("Send Route ", origin, "to ", peer_to_send,
                "id", gossiper.VectorTable[origin])
    packet := &message.RumorMessage{Origin:origin,
                                    ID:gossiper.VectorTable[origin],
                                    Text:"",
                                    PublicKey: security.PUBLIC_KEY_TEXT}
    fmt.Println("MONGERING ROUTE to", peer_to_send)
    log.Println("MONGERING ROUTE to", peer_to_send)
    gossiper.SendRumor(packet, peer_to_send)
}

func (gossiper *Gossiper) RouteRumor(){
    gossiper.RouteRumorInit()
    for {
        timer := time.NewTimer(time.Duration(RTIMER)*time.Second)
        <-timer.C
        var peer_map map[string]bool
        peer_map= gossiper.PeerList
        peer_list := []string{}
        for peer, _ := range(peer_map) {
            peer_list = append(peer_list, peer) }
        if len(peer_list) == 0 { continue }
        peer_to_send := peer_list[rand.Intn(len(peer_list))]
        gossiper.SendRoute(gossiper.Name, peer_to_send)
    }
}

func (gossiper *Gossiper) RouteRumorInit(){
    for peer_to_send, _ := range(gossiper.PeerList) {
        gossiper.SendRoute(gossiper.Name, peer_to_send)
    }
}

//func (gossiper *Gossiper) Close() {
//    gossiper.UIConn.Close()
//    gossiper.GossipConn.Close()
//}

func main() {
    // Parse the flags.
    var ui_port = flag.String("UIPort", "",
                    "UI Port where the gossip program listens")
    var gossipPort = flag.String("gossipAddr", "",
                    "Address for the current gossiper")
    var name = flag.String("name", "", "Name of the current host")
    var peers = flag.String("peers", "",
                    "List of peers in the form <ip>:port separated by an underscore")
    var webport = flag.String("webport", "8080", "Port for local web client")
    var rtimer = flag.Int("rtimer", 60, "Number of seconds to wait between 2 rumor messsages")
    var noforward = flag.Bool("noforward", false, "Set if the node should not forward messages to other nodes")
    var chunk_size = flag.Int64("chunk", 8*(1<<10), "File chunk size")
	var streaming_size = flag.Int64("streaming", 1024*(1<<10), "File streaming size")
	var encoding = flag.Bool("encoding", false, "Set if the node should encode video files")

    flag.Parse()

    rand.Seed(time.Now().UTC().UnixNano())
    DEBUG=true
    PACKET_SIZE = 10*1024
    HOPLIMIT = 10
    BUDGET_THRESHOLD = 32
    MATCH_THRESHOLD = 2
    MessageQueue = make(chan message.Message)
    WebServerReceiveChannel = make(chan message.ClientMessage)
    WebServerSendChannel = make(chan message.ClientMessage)
    RTIMER = *rtimer
    NOFORWARD = *noforward
    GOSSIP_PORT = *gossipPort
    UI_PORT =*ui_port
    CHUNK_SIZE = *chunk_size
    STREAMING_SIZE = *streaming_size
    ENCODING = *encoding

    dir, err := ioutil.TempDir(".", GOSSIP_PORT)
    if err != nil {
        fmt.Println("Failed to create temporary directory", err)
        log.Println("Failed to create temporary directory", err)
        os.Exit(1)
    }
    defer os.RemoveAll(dir)
    SHARING_DIRECTORY = dir

    //create your file with desired read/write permissions
    logfile := *name + ".log"
    f, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
    if err != nil {
            log.Fatal(err)
    }
    defer f.Close()

    //set output of logs to f
    log.SetOutput(f)

    //initialise security keys
    security.InitKeys(dir, *name)

    //test case
    log.Println("check to make sure it works")
    var peer_list []string
    if *peers == "" { peer_list = []string{}
    } else { peer_list = strings.Split(*peers, "_") }

    gossiper := NewGossiper(*name, *webport, peer_list)
    //defer gossiper.Close()

    go gossiper.SendStatus()
    go gossiper.GossipMessages()
    go gossiper.GossipRequestMessages()
    go gossiper.GossipReplyMessages()
    go gossiper.GossipPrivateMessages()
    go gossiper.GossipSearchRequests()
    go gossiper.GossipSearchReply()
    go gossiper.WaitForAck()
    go gossiper.WaitForReply()
    go gossiper.WaitForSRequest()
    go gossiper.WaitForSReply()
    go gossiper.AntiEntropy()
    go gossiper.RouteRumor()

    sigs := make(chan os.Signal, 1)
    done := make(chan bool, 1)

    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        sig := <-sigs
        fmt.Println()
        fmt.Println(sig)
        log.Println(sig)
        done <- true
    }()
    <-done
}
