package main

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "flag"
    "io/ioutil"
    "math/rand"
    "net"
    "os"
    "os/signal"
    "path/filepath"
    "strings"
    "strconv"
    "syscall"
    "time"

    "github.com/dedis/protobuf"
    "github.com/Swarali/Peerster/part1/message"
    "github.com/Swarali/Peerster/part1/webserver"
    )

// go run gossiper.go -UIPort=10000 -gossipAddr=127.0.0.1:5000 -name=nodeA -peers=127.0.0.1:5001_10.1.1.7:5002
var DEBUG bool
var RTIMER int
var NOFORWARD bool
var HOPLIMIT uint32
var UI_PORT string
var GOSSIP_PORT string
var CHUNK_SIZE int64
var SHARING_DIRECTORY string
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
    Hash []byte
    Reply_wait chan message.DataReply
    Close bool}

type UploadedFile struct {
    FileSize int64
    MetaFileName string
    MetaHash []byte
}

type Gossiper struct {
    Name string
    UIConn *net.UDPConn
    GossipConn *net.UDPConn
    RumorChannel chan message.GossipMessage
    StatusChannel chan message.GossipMessage
    PrivateChannel chan message.GossipMessage
    RequestChannel chan message.GossipMessage
    ReplyChannel chan message.GossipMessage
    VectorTable map[string]uint32
    NextRoutingTable map[string]string
    NextRoutingSeq map[string]uint32
    RumorMessages map[string][]message.RumorMessage
    PrivateMessages map[string][]string
    UploadedFiles map[string]UploadedFile
    UploadedChunks map[string][]string
    PeerList map[string]bool
    AckReceiveChannel chan string
    AckSendChannel chan AckWait
    ReplyWaitChannel chan ReplyWait
    ReplyReceiveChannel chan message.DataReply
}

func NewGossiper(name, webport string,
                 peer_list []string) *Gossiper {
    peer_list_map := make(map[string]bool)
    for _, peer := range peer_list { peer_list_map[peer] = true }
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
                          VectorTable: make(map[string]uint32),
                          NextRoutingTable: make(map[string]string),
                          NextRoutingSeq: make(map[string]uint32),
                          RumorMessages: make(map[string][]message.RumorMessage),
                          PrivateMessages: make(map[string][]string),
                          UploadedFiles: make(map[string]UploadedFile),
                          PeerList: peer_list_map,
                          AckReceiveChannel: make(chan string),
                          AckSendChannel: make(chan AckWait),
                          ReplyWaitChannel: make(chan ReplyWait),
                          ReplyReceiveChannel: make(chan message.DataReply),
                        }
    //Add self into the VectorTable
    gossiper.VectorTable[name] = 1
    if DEBUG { fmt.Println("Start receiving client msg on", UI_PORT, ":", client_conn) }
    go ReceiveClientMessage(client_conn)

    if DEBUG { fmt.Println("Start receiving gossip msg on", GOSSIP_PORT, ":", gossip_conn) }
    go ReceiveGossipMessage(gossip_conn)

    if DEBUG { fmt.Println("Start receiving webclient msg on", webport) }
    go ReceiveWebClientMessage(name, peer_list, webport)

    go gossiper.ProcessMessageQueue()
    return gossiper
}

func ReceiveClientMessage(conn *net.UDPConn) {
    if DEBUG { fmt.Println("Receive messages on", conn) }
    for {
        packet := &message.ClientMessage{}
        packetBytes := make([]byte, 1024)
        rlen, _, err := conn.ReadFrom(packetBytes)
        if err != nil {
            if DEBUG { fmt.Println(err) }
            continue
        }
        if rlen > 1024 {
            if DEBUG { fmt.Println("Size error: ", rlen) }
            continue
        }
        protobuf.Decode(packetBytes, packet)
        if DEBUG { fmt.Println("Received message on", packet) }
        MessageQueue<-message.Message{ClientMsg: packet}
    }
}

func ReceiveGossipMessage(conn *net.UDPConn) {
    if DEBUG { fmt.Println("Receive messages on", conn) }
    for {
        packet := &message.GossipPacket{}
        packetBytes := make([]byte, 1024)
        rlen, addr, err := conn.ReadFrom(packetBytes)
        if err != nil {
            if DEBUG { fmt.Println(err) }
            continue
        }
        if rlen > 1024 {
            if DEBUG { fmt.Println("Size error: ", rlen) }
            continue
        }
        protobuf.Decode(packetBytes, packet)
        relay_addr := addr.String()
        if relay_addr == GOSSIP_PORT {
            fmt.Println("Drop packets from the same node")
            continue
        }
        gossip_msg := &message.GossipMessage{Packet:*packet,
                                            Relay_addr:relay_addr}
        MessageQueue<-message.Message{GossipMsg: gossip_msg}
    }
}

func ReceiveWebClientMessage(name string, peer_list []string, webport string) {
    webserver.WebServerReceiveChannel = WebServerReceiveChannel
    webserver.WebServerSendChannel = WebServerSendChannel
    go webserver.StartWebServer(name, peer_list, webport)
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
                if gossip_packet.Rumor.LastIP != nil && gossip_packet.Rumor.LastPort != nil {
                gossiper.UpdatePeer(gossip_packet.Rumor.LastIP.String()+":"+strconv.Itoa(*gossip_packet.Rumor.LastPort))
            }
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
            }
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
            } else if operation == "NewFileUpload" {
                go gossiper.UploadFile(client_msg)
                continue
            } else if operation == "NewFileDownload" {
                file_hash := msg.ClientMsg.HashValue
                destination := msg.ClientMsg.Destination
                go gossiper.DownloadFile(client_msg, destination,
                                         file_hash)
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
                gossip_packet.Private.Text = client_msg
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
        } else {
            fmt.Printf("RUMOR origin %s from %s ID %d contents %s\n", packet.Rumor.Origin, relay_addr, packet.Rumor.ID,
                            packet.Rumor.Text)
        }
    } else if packet.Status != nil {
        status_str := fmt.Sprintf("STATUS from %s", relay_addr)
        for _, peer_status := range packet.Status.Want {
            peer_status_str := fmt.Sprintf("origin %s nextID %d", peer_status.Identifier, peer_status.NextID)
            status_str = fmt.Sprintf("%s %s", status_str, peer_status_str)
        }
        fmt.Println(status_str)
    } else if packet.Private != nil {
        private_msg := packet.Private
        if private_msg.Destination == gossiper.Name {
            fmt.Printf("PRIVATE: %s:%d:%s\n", private_msg.Origin,
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
}

func (gossiper *Gossiper)UpdatePeer(relay_addr string) {
    if relay_addr == "N/A" { return }
    if relay_addr == GOSSIP_PORT { return }
    _, peer_exists := gossiper.PeerList[relay_addr]
    if peer_exists == false {
        if DEBUG { fmt.Println("Updating Peer: ", relay_addr) }
        gossiper.PeerList[relay_addr] = true
        WebServerSendChannel<-message.ClientMessage{Operation:"NewPeer", Message: relay_addr}
    }
}

func (gossiper *Gossiper)UpdateID(new_id string) {
    gossiper.Name = new_id
    gossiper.VectorTable[new_id] = 1
    WebServerSendChannel<-message.ClientMessage{Operation:"NewID", Message: new_id}
    gossiper.RouteRumorInit()
}

func (gossiper *Gossiper) UploadFile(file_name string) {
    file_path := filepath.Join("files", file_name)
    file, err := os.Open(file_path)
    if err != nil {
        fmt.Println("Error opening file:", file_name, err)
    }
    defer file.Close()
    file_info, _ := file.Stat()
    file_size := file_info.Size()

    metafile_name := "meta_"+file_name
    metafile_path := filepath.Join(SHARING_DIRECTORY, metafile_name)
    metafile, err := os.Create(metafile_path)
    if err != nil {
        fmt.Println("Unable to create metafile", metafile_path)
        return
    }
    defer metafile.Close()

    offset := int64(0)
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
            break
        }
        chunk_file.Close()

        //Calculate and append SHA of the chunk
        h := sha256.New()
        h.Write(chunk)
        chunk_hash := h.Sum(nil)
        metafile.Write(chunk_hash)
        metadata_hash.Write(chunk_hash)

        offset +=file_chunk_size
        chunk_no+=1

    }

    //Calculate SHA of metafile
    metafile_hash := metadata_hash.Sum(nil)
    gossiper.UploadedFiles[file_name] = UploadedFile{FileSize: file_size,
                                        MetaFileName: metafile_name,
                                        MetaHash: metafile_hash }
    if DEBUG { fmt.Println("Uploaded and chunked file:", file_name, ":", gossiper.UploadedFiles[file_name]) }
    WebServerSendChannel<-message.ClientMessage{Operation:"NewFileUpload", Message: file_name}
}

func (gossiper *Gossiper) DownloadFile(file_name string, destination string, file_hash []byte) {
    data_request := message.DataRequest{Origin: gossiper.Name,
                                        Destination: destination,
                                        HopLimit: HOPLIMIT,
                                        FileName: file_name,
                                        HashValue: file_hash}
    //Send Request.
    fmt.Println("Downloading metafile of", file_name, "from", destination)
    gossiper.RequestChannel<-message.GossipMessage{Packet: message.GossipPacket{Request: &data_request}, Relay_addr: "N/A"}
    meta_reply := gossiper.GetReply(file_hash)
    if file_name != meta_reply.FileName || hex.EncodeToString(file_hash) != hex.EncodeToString(meta_reply.HashValue) {
        fmt.Println("File Name or Hash Value does not match")
    }
    metadata_hash := sha256.New()
    metadata_hash.Write(meta_reply.Data)
    if hex.EncodeToString(file_hash)!= hex.EncodeToString(metadata_hash.Sum(nil)){
        fmt.Println("Hash of the data does not match Hash Value")
    }
    metafile_name := "meta_"+file_name
    metafile_path := filepath.Join(SHARING_DIRECTORY, metafile_name)
    metafile, err := os.Create(metafile_path)
    if err != nil {
        fmt.Println("Unable to create metafile", metafile_path)
        return
    }
    metafile.Write(meta_reply.Data)
    metafile.Close()
    gossiper.UploadedChunks[file_name] = make([]string, (len(meta_reply.Data)/32)+1)
    gossiper.UploadedChunks[file_name] = append(gossiper.UploadedChunks[file_name], hex.EncodeToString(file_hash))

    file_path := filepath.Join("files", file_name)
    file, err := os.Create(file_path)
    if err != nil {
        fmt.Println("Unable to create file", file_path)
        return
    }
    defer file.Close()

    chunk_no := 0
    file_size := int64(0)
    for {
        offset := chunk_no*32
        chunk_hash_req := meta_reply.Data[offset:offset+32]
        chunk_request := message.DataRequest{Origin: gossiper.Name,
                                            Destination: destination,
                                            HopLimit: HOPLIMIT,
                                            FileName: file_name,
                                            HashValue: chunk_hash_req}
        //Send Request.
        fmt.Println("Downloading", file_name, "chunk", chunk_no+1, "from", destination)
        gossiper.RequestChannel<-message.GossipMessage{Packet: message.GossipPacket{Request: &chunk_request}, Relay_addr: "N/A"}
        reply := gossiper.GetReply(file_hash)
        if file_name != reply.FileName || hex.EncodeToString(chunk_hash_req) != hex.EncodeToString(reply.HashValue) {
            fmt.Println("File Name or Hash Value does not match")
            continue
        }
        chunk_hash := sha256.New()
        chunk_hash.Write(reply.Data)
        if hex.EncodeToString(chunk_hash_req) != hex.EncodeToString(chunk_hash.Sum(nil)) {
            fmt.Println("Hash of the data does not match Hash Value")
            continue
        }
        chunk_path := filepath.Join(SHARING_DIRECTORY, "chunk_"+strconv.Itoa(chunk_no+1)+"_"+file_name)
        chunkfile, err := os.Create(chunk_path)
        if err != nil {
            fmt.Println("Unable to create chunkfile", chunk_path)
            return
        }
        chunkfile.Write(reply.Data)
        chunkfile.Close()
        gossiper.UploadedChunks[file_name] = append(gossiper.UploadedChunks[file_name], hex.EncodeToString(chunk_hash_req))

        file.Write(reply.Data)
        chunk_no +=1
        file_size+=int64(len(reply.Data))
        if len(meta_reply.Data) >= chunk_no*32 { break }
    }
    fmt.Println("RECONSTRUCTED file", file_name)
    gossiper.UploadedFiles[file_name] = UploadedFile{FileSize: file_size,
                                        MetaFileName: metafile_name,
                                        MetaHash: meta_reply.Data}
    WebServerSendChannel<-message.ClientMessage{Operation:"NewFileUpload", Message: file_name}
}

func (gossiper *Gossiper) GetReply(file_hash []byte) message.DataReply {
    for {
        // Wait for reply or timeout
        timer := time.NewTimer(5*time.Second)
        reply_wait := make(chan message.DataReply, 1)
        reply_wait_obj := ReplyWait{Hash: file_hash,
                                    Reply_wait: reply_wait}
        gossiper.ReplyWaitChannel <- reply_wait_obj
        select {
            case reply := <-reply_wait:
                if DEBUG { fmt.Println("Reply received") }
                reply_wait_obj.Close = true
                gossiper.ReplyWaitChannel<-reply_wait_obj
                return reply
            case <-timer.C:
                if DEBUG { fmt.Println("Timeout") }
                // De-register the object from the wait.
                reply_wait_obj.Close = true
                gossiper.ReplyWaitChannel<-reply_wait_obj
        }
    }

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
                if DEBUG {
                    fmt.Println("Not forwarding messages since noforward flag is set") }
            } else {
                fmt.Println("MONGERING TEXT to "+relay_addr)
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
            fmt.Println("Packet:", packet.Origin, packet.ID, packet.Text, *packet.LastIP, *packet.LastPort)
        }

        next_id, ok := gossiper.VectorTable[packet.Origin]
        if !ok {
            //gossiper.RumorMessages[packet.Origin] = make(string[])
            gossiper.VectorTable[packet.Origin] = 1
            next_id = 1
        }

        gossiper.UpdateRoutingTable(channel_packet)
        if packet.Text == "" {
            var peer_to_send string
            var peer_list []string
            last_relay := channel_packet.Relay_addr
            for peer, _ := range gossiper.PeerList {
                if peer != last_relay {
                peer_list = append(peer_list, peer) }
            }
            if len(peer_list) != 0 {
                peer_to_send = peer_list[rand.Intn(len(peer_list))]
                if DEBUG { fmt.Println("Send Route ", packet.Origin, *packet.LastIP, *packet.LastPort, "to ", peer_to_send) }
                fmt.Println("MONGERING ROUTE to "+peer_to_send)
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
            if DEBUG {gossiper.PrintPeers()
                      fmt.Println("RumorMessages:", gossiper.RumorMessages,
                        "Vector Table:", gossiper.VectorTable)}

            if NOFORWARD { if DEBUG { fmt.Println("Not forwarding messages since noforward flag is set") }
            } else { go gossiper.TransmitMessage(channel_packet,
                                                     gossiper.PeerList)
            }
        } else {
            //Discard the message since it is not in order
            if DEBUG {
                fmt.Println("Discarding the message", packet.ID,
                            packet.Origin, packet.Text)
                gossiper.PrintPeers()
                fmt.Println("RumorMessages:", gossiper.RumorMessages,
                            "Vector Table:", gossiper.VectorTable)
            }
        }

        if relay_addr != "N/A" { go gossiper.SendAck(relay_addr)}
    }
}

func (gossiper *Gossiper) IsChunkPresent(req message.DataRequest, relay_addr string) bool{
    file_name := req.FileName
    hash_value := req.HashValue
    chunk_hashes, ok := gossiper.UploadedChunks[file_name]
    if !ok {
        return false
    }

    for chunk_no, chunk_hash := range chunk_hashes {
        if chunk_hash == hex.EncodeToString(hash_value) {
            go gossiper.SendReply(req.Origin, file_name, chunk_no, relay_addr)
            return true
        }
    }
    return false
}

func (gossiper *Gossiper) GossipReplyMessages() {
    for channel_packet := range gossiper.ReplyChannel {
        // relay_addr := channel_packet.Relay_addr
        packet  := channel_packet.Packet.Reply
        if packet.Destination == gossiper.Name {
            if DEBUG { fmt.Println("File", packet.FileName, "reply from", packet.Origin) }
            gossiper.ReplyReceiveChannel<-*packet
            continue
        }
        if packet.HopLimit <=0 {
            // Discard the packet.
            if DEBUG { fmt.Println("Discarding reply message due to hop limit exceeded:",
                                   packet.Origin, packet.FileName,
                                   packet.HopLimit,
                                   packet.Destination) }
            continue
        }
        next_addr, ok := gossiper.NextRoutingTable[packet.Destination]
        if !ok {
            // Discard the packet.
            if DEBUG { fmt.Println("Discarding reply message due to no entry in routing table:",
                                   packet.Origin, packet.FileName,
                                   packet.HopLimit,
                                   packet.Destination) }
            continue
        }
        if NOFORWARD { fmt.Println("Not forwarding reply message")
        } else { go gossiper.SendReplyMessage(packet, next_addr) }
    }
}

func (gossiper *Gossiper) GossipRequestMessages() {
    for channel_packet := range gossiper.RequestChannel {
        relay_addr := channel_packet.Relay_addr
        packet  := channel_packet.Packet.Request
        if gossiper.IsChunkPresent(*packet, relay_addr) {
            if DEBUG { fmt.Println("Found the requested chunk") }
            continue
        }
        if packet.Destination == gossiper.Name {
            if DEBUG { fmt.Println("File", packet.FileName, "cannot be found on", gossiper.Name) }
            continue
        }
        if packet.HopLimit <=0 {
            // Discard the packet.
            if DEBUG { fmt.Println("Discarding request message due to hop limit exceeded:",
                                   packet.Origin, packet.FileName,
                                   packet.HopLimit,
                                   packet.Destination) }
            continue
        }
        next_addr, ok := gossiper.NextRoutingTable[packet.Destination]
        if !ok {
            // Discard the packet.
            if DEBUG { fmt.Println("Discarding request message due to no entry in routing table:",
                                   packet.Origin, packet.FileName,
                                   packet.HopLimit,
                                   packet.Destination) }
            continue
        }
        if NOFORWARD { fmt.Println("Not forwarding request message")
        } else { go gossiper.SendRequestMessage(packet, next_addr) }
    }
}

func (gossiper *Gossiper) GossipPrivateMessages() {
    for channel_packet := range gossiper.PrivateChannel {
        // relay_addr := channel_packet.Relay_addr
        packet  := channel_packet.Packet.Private
        if packet.Destination == gossiper.Name {
            WebServerSendChannel<-message.ClientMessage{Operation:"NewPrivateMessage", Message:packet.Text, Origin:packet.Origin}
            continue
        }
        if packet.Origin == gossiper.Name {
            WebServerSendChannel<-message.ClientMessage{Operation:"NewPrivateMessage", Message:packet.Text, Destination:packet.Destination}
        }
        if packet.HopLimit <=0 {
            // Discard the packet.
            if DEBUG { fmt.Println("Discarding private message due to hop limit exceeded:",
                                   packet.Origin, packet.Text,
                                   packet.HopLimit,
                                   packet.Destination) }
            continue
        }
        next_addr, ok := gossiper.NextRoutingTable[packet.Destination]
        if !ok {
            // Discard the packet.
            if DEBUG { fmt.Println("Discarding private message due to no entry in routing table:",
                                   packet.Origin, packet.Text,
                                   packet.HopLimit,
                                   packet.Destination) }
            continue
        }
        if NOFORWARD { fmt.Println("Not forwarding private message")
        } else { go gossiper.SendPrivateMessage(packet, next_addr) }
    }
}

func (gossiper *Gossiper) UpdateRoutingTable(channel_packet message.GossipMessage) {
    relay_addr := channel_packet.Relay_addr
    packet  := channel_packet.Packet.Rumor
    if relay_addr == "N/A" { return }
    if DEBUG { fmt.Println("Received Route rumor about ", packet.Origin, " from ", relay_addr) }
    next_seq, ok := gossiper.NextRoutingSeq[packet.Origin]
    is_direct:=false
    if next_seq == packet.ID {
        if packet.LastIP == nil && packet.LastPort == nil && (!ok || (next_seq <= packet.ID)) {
            is_direct=true
        }
    }
    if !ok || (next_seq < packet.ID) || is_direct {
        if is_direct {
            fmt.Printf("DIRECT-ROUTE FOR %s: %s\n", packet.Origin, relay_addr) }
        gossiper.NextRoutingSeq[packet.Origin] = packet.ID
        if relay_addr != "N/A" {
            _, ok := gossiper.NextRoutingTable[packet.Origin]
            gossiper.NextRoutingTable[packet.Origin] = relay_addr
            if !ok {
                WebServerSendChannel<-message.ClientMessage{Operation: "NewRoute", Message: packet.Origin}
            }
            fmt.Printf("DSDV %s: %s\n", packet.Origin, relay_addr)
        }
    }
}

func (gossiper *Gossiper) TransmitMessage(channel_packet message.GossipMessage,
                                          peer_list_map map[string]bool) {
    if DEBUG { fmt.Println("Sending message") }
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
                    if DEBUG { fmt.Println("Ack received") }
                case <-timer.C:
                    if DEBUG { fmt.Println("Timeout") }
                    // De-register the object from the wait.
                    gossiper.AckSendChannel<-ack_wait_obj
            }
        }
    }

}
func (gossiper *Gossiper) TransmitMessageWithRumorMongering(channel_packet message.GossipMessage,
                                          peer_list_map map[string]bool) {
    if DEBUG { fmt.Println("Sending message") }
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
            gossiper.PrintPeers()
        } else {
            fmt.Println("MONGERING TEXT to "+peer_to_send)
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
                if DEBUG { fmt.Println("Ack received") }
            case <-timer.C:
                if DEBUG { fmt.Println("Timeout") }
                // De-register the object from the wait.
                gossiper.AckSendChannel<-ack_wait_obj
        }

        if (rand.Int() % 2) == 0 { break }
        flipped_coin = true
    }
}

func (gossiper *Gossiper)SendGossip(packet message.GossipPacket,
                                    peer_to_send string) {
    if DEBUG { fmt.Println("Peer to send is ", peer_to_send, "Packet to send", packet) }
    packetBytes, _ := protobuf.Encode(&packet)
    udpAddr, _ := net.ResolveUDPAddr("udp", peer_to_send)
    gossiper.GossipConn.WriteToUDP(packetBytes, udpAddr)
}

func (gossiper *Gossiper)SendRumor(rumor *message.RumorMessage, peer_to_send string) {
    if DEBUG { fmt.Println("Send Rumor", rumor.Origin, rumor.ID, rumor.Text, rumor.LastIP, rumor.LastPort) }
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
    if DEBUG { fmt.Println("Send Ack", status_packet) }
    gossiper.SendGossip(message.GossipPacket{Status: &status_packet},
                        relay_addr)
}

func (gossiper *Gossiper)SendPrivateMessage(private_msg *message.PrivateMessage, next_addr string) {
    if DEBUG { fmt.Println("Send PM", private_msg) }
    gossiper.SendGossip(message.GossipPacket{Private: private_msg}, next_addr)
}

func (gossiper *Gossiper)SendRequestMessage(req *message.DataRequest, next_addr string) {
    if DEBUG { fmt.Println("Send Request", req) }
    gossiper.SendGossip(message.GossipPacket{Request: req}, next_addr)
}

func (gossiper *Gossiper) SendReplyMessage(reply *message.DataReply, relay_addr string) {
    if DEBUG { fmt.Println("Send Reply", reply) }
    gossiper.SendGossip(message.GossipPacket{Reply: reply}, relay_addr)

}

func (gossiper *Gossiper) SendReply(destination string, file_name string, chunk_no int, relay_addr string) {
    chunk_reply := message.DataReply{Origin: gossiper.Name, Destination:destination, FileName: file_name, HopLimit: HOPLIMIT}
    var path string
    if chunk_no == 0 {
        // Return metadata file
        path = filepath.Join(SHARING_DIRECTORY, "meta_"+file_name)
    } else {
        path =filepath.Join(SHARING_DIRECTORY, "chunk_"+strconv.Itoa(chunk_no)+"_"+file_name)
    }
    file, _ := os.Open(path)
    defer file.Close()
    data := make([]byte, CHUNK_SIZE)
    file.Read(data)
    chunk_reply.Data = data
    hash := gossiper.UploadedChunks[file_name][chunk_no]
    chunk_reply.HashValue, _ = hex.DecodeString(hash)

    if DEBUG { fmt.Println("Send Reply", chunk_reply) }
    gossiper.SendReplyMessage(&chunk_reply, relay_addr)
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
    wait_map := make(map[string] chan message.DataReply)
    for {
        select {
            case reply := <-gossiper.ReplyReceiveChannel:
                hash_string := hex.EncodeToString(reply.HashValue)
                channel, ok := wait_map[hash_string]
                if ok {
                    channel <- reply
                }
            case req := <-gossiper.ReplyWaitChannel:
                hash_string := hex.EncodeToString(req.Hash)
                reply_wait := req.Reply_wait
                close_chan := req.Close
                // If timeout occurs we send back req_wait to 
                // de-register it from wait_map.
                old_ack_chan, old_ack_ok := wait_map[hash_string]
                if close_chan && old_ack_ok {
                    delete(wait_map, hash_string)
                    close(old_ack_chan)
                } else {
                    wait_map[hash_string] = reply_wait
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
    if DEBUG { fmt.Println("Send Route ", origin, "to ", peer_to_send, "id", gossiper.VectorTable[origin]) }
    packet := &message.RumorMessage{Origin:origin,
                                    ID:gossiper.VectorTable[origin],
                                    Text:""}
    fmt.Println("MONGERING ROUTE to", peer_to_send)
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

func (gossiper *Gossiper) Close() {
    gossiper.UIConn.Close()
    gossiper.GossipConn.Close()
}

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
    flag.Parse()

    rand.Seed(time.Now().UTC().UnixNano())
    DEBUG=true
    HOPLIMIT = 10
    MessageQueue = make(chan message.Message)
    WebServerReceiveChannel = make(chan message.ClientMessage)
    WebServerSendChannel = make(chan message.ClientMessage)
    RTIMER = *rtimer
    NOFORWARD = *noforward
    GOSSIP_PORT = *gossipPort
    UI_PORT =*ui_port
    CHUNK_SIZE = *chunk_size
    dir, err := ioutil.TempDir("", GOSSIP_PORT)
    if err != nil {
        fmt.Println("Failed to create temporary directory", err)
        os.Exit(1)
    }
    defer os.RemoveAll(dir)
    SHARING_DIRECTORY = dir

    var peer_list []string
    if *peers == "" { peer_list = []string{}
    } else { peer_list = strings.Split(*peers, "_") }

    gossiper := NewGossiper(*name, *webport, peer_list)
    defer gossiper.Close()

    go gossiper.SendStatus()
    go gossiper.GossipRequestMessages()
    go gossiper.GossipPrivateMessages()
    go gossiper.WaitForAck()
    go gossiper.WaitForReply()
    go gossiper.AntiEntropy()
    go gossiper.RouteRumor()
    go gossiper.GossipMessages()

    sigs := make(chan os.Signal, 1)
    done := make(chan bool, 1)

    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        sig := <-sigs
        fmt.Println()
        fmt.Println(sig)
        done <- true
    }()
    <-done
}
