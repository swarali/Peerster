package main

import (
    "fmt"
    "flag"
    "math/rand"
    "net"
    "strings"
    //"strconv"
    "time"

    "github.com/dedis/protobuf"
    "github.com/Swarali/Peerster/part2/message"
    "github.com/Swarali/Peerster/part2/webserver"
    )

// go run gossiper.go -UIPort=10000 -gossipPort=5000 -name=nodeA -peers=127.0.0.1:5001_10.1.1.7:5002
var DEBUG bool
var RTIMER int
var HOPLIMIT uint32
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
    Ack_wait chan bool }

type Gossiper struct {
    Name string
    UIPort string
    UIConn *net.UDPConn
    GossipPort string
    GossipConn *net.UDPConn
    RumorChannel chan message.GossipMessage
    StatusChannel chan message.GossipMessage
    PrivateChannel chan message.GossipMessage
    VectorTable map[string]uint32
    NextRoutingTable map[string]string
    RumorMessages map[string][]string
    PrivateMessages map[string][]string
    PeerList map[string]bool
    AckReceiveChannel chan string
    AckSendChannel chan AckWait
}

func NewGossiper(name, ui_port, gossip_port, webport string,
                 peer_list []string) *Gossiper {
    peer_list_map := make(map[string]bool)
    for _, peer := range peer_list { peer_list_map[peer] = true }
    client_conn := ListenOn("127.0.0.1:" + ui_port)
    gossip_conn := ListenOn(gossip_port)

    //vectorTable := map[string]int{name:1}
    gossiper:= &Gossiper{Name: name,
                          UIPort: ui_port,
                          UIConn: client_conn,
                          GossipPort: gossip_port,
                          GossipConn: gossip_conn,
                          RumorChannel: make(chan message.GossipMessage),
                          StatusChannel: make(chan message.GossipMessage),
                          PrivateChannel: make(chan message.GossipMessage),
                          VectorTable: make(map[string]uint32),
                          NextRoutingTable: make(map[string]string),
                          RumorMessages: make(map[string][]string),
                          PrivateMessages: make(map[string][]string),
                          PeerList: peer_list_map,
                          AckReceiveChannel: make(chan string),
                          AckSendChannel: make(chan AckWait),
                        }
    //Add self into the VectorTable
    gossiper.VectorTable[name] = 1
    if DEBUG { fmt.Println("Start receiving client msg on", ui_port) }
    go ReceiveClientMessage(client_conn)

    if DEBUG { fmt.Println("Start receiving gossip msg on", gossip_port) }
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
        operation := message_from_webclient.Operation
        msg := message_from_webclient.Message
        client_msg := message.ClientMessage{}
        if operation == "NewID" || operation == "NewMessage" || operation == "NewPeer" {
            client_msg.Message = msg
            client_msg.Operation = operation
        } else if operation == "NewPrivateMessage" {
            client_msg.Message = msg
            client_msg.Operation = operation
            client_msg.Destination = message_from_webclient.Destination
        }
        MessageQueue<-message.Message{ClientMsg:&client_msg}
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
            } else if gossip_packet.Status != nil {
                channel = gossiper.StatusChannel
            } else if gossip_packet.Private != nil {
                gossip_packet.Private.HopLimit-=1
                channel = gossiper.PrivateChannel
            }
        } else {
            operation := msg.ClientMsg.Operation
            client_msg := msg.ClientMsg.Message
            if operation == "NewID" {
                gossiper.UpdateID(client_msg)
                continue
            } else if operation == "NewPeer" {
                gossiper.UpdatePeer(client_msg)
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
        gossiper.UpdatePeer(relay_addr)
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
    _, peer_exists := gossiper.PeerList[relay_addr]
    if peer_exists == false {
        gossiper.PeerList[relay_addr] = true
        WebServerSendChannel<-message.ClientMessage{Operation:"NewPeer", Message: relay_addr}
    }
}

func (gossiper *Gossiper)UpdateID(new_id string) {
    gossiper.Name = new_id
    gossiper.VectorTable[new_id] = 1
    WebServerSendChannel<-message.ClientMessage{Operation:"NewID",                                                 Message: new_id}
    gossiper.RouteRumorInit()
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
                rumor_msg = &message.RumorMessage{Origin: origin,
                                                  ID: peer_next_id,
                                                  Text: gossiper.RumorMessages[origin][peer_next_id-1]}
                break
            }
        }
        if gossiper_has_new_msg {
            fmt.Println("MONGERING with "+relay_addr)
            gossiper.PrintPeers()
            go gossiper.SendRumor(rumor_msg, relay_addr)
            //continue
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
        }

        next_id, ok := gossiper.VectorTable[packet.Origin]
        if !ok {
            //gossiper.RumorMessages[packet.Origin] = make(string[])
            gossiper.VectorTable[packet.Origin] = 1
            next_id = 1
        }

        if packet.ID == next_id {
            // Update messages only if the message is non-empty.
            if packet.Text != "" {
                message_list := append(gossiper.RumorMessages[packet.Origin],
                                       packet.Text)
                gossiper.RumorMessages[packet.Origin] = message_list
                gossiper.VectorTable[packet.Origin] += 1
                // Notify the WebServer.
                WebServerSendChannel<-message.ClientMessage{Operation:"NewMessage", Message:packet.Text, Origin:packet.Origin}
            }
            gossiper.UpdateRoutingTable(channel_packet)
            if DEBUG {gossiper.PrintPeers()
                      fmt.Println("RumorMessages:", gossiper.RumorMessages,
                        "Vector Table:", gossiper.VectorTable)}

            go gossiper.TransmitMessage(channel_packet,
                                        gossiper.PeerList)
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
        go gossiper.SendPrivateMessage(packet, next_addr)
    }
}

func (gossiper *Gossiper) UpdateRoutingTable(channel_packet message.GossipMessage) {
    relay_addr := channel_packet.Relay_addr
    if relay_addr != "N/A" {
        packet  := channel_packet.Packet.Rumor
        _, ok := gossiper.NextRoutingTable[packet.Origin]
        gossiper.NextRoutingTable[packet.Origin] = relay_addr
        if !ok {
            WebServerSendChannel<-message.ClientMessage{Operation: "NewRoute", Message: packet.Origin}
        }
        fmt.Printf("DSDV %s:%s\n", packet.Origin, relay_addr)
    }
}

func (gossiper *Gossiper) TransmitMessage(channel_packet message.GossipMessage,
                                          peer_list_map map[string]bool) {
    if DEBUG { fmt.Println("Sending message") }
    var peer_list []string
    for peer, _ := range peer_list_map { peer_list = append(peer_list, peer) }
    last_relay := channel_packet.Relay_addr
    packet := channel_packet.Packet

    // Pick up a random peer
    var peer_to_send string
    var flipped_coin bool
    for {
        // If only one peer known do not do this for loop
        if len(peer_list) == 1 { return }
        for {
            peer_to_send = peer_list[rand.Intn(len(peer_list))]
            if peer_to_send != last_relay { break }
        }

        if flipped_coin {
            fmt.Println("FLIPPED COIN sending rumor to "+peer_to_send)
            gossiper.PrintPeers()
        } else {
            fmt.Println("MONGERING with "+peer_to_send)
            gossiper.PrintPeers()
        }
        gossiper.SendRumor(packet.Rumor, peer_to_send)

        // Wait for ack or timeout
        timer := time.NewTimer(time.Second)
        ack_wait := make(chan bool, 1)
        ack_id := packet.Rumor.Origin + ":" + fmt.Sprint(packet.Rumor.ID)
        ack_wait_obj := AckWait{Addr: peer_to_send,
                                Ack_id : ack_id,
                                Ack_wait: ack_wait}
        gossiper.AckSendChannel<- ack_wait_obj
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
    if DEBUG { fmt.Println("Peer to send is ", peer_to_send) }
    packetBytes, _ := protobuf.Encode(&packet)
    udpAddr, _ := net.ResolveUDPAddr("udp", peer_to_send)
    gossiper.GossipConn.WriteToUDP(packetBytes, udpAddr)
}

func (gossiper *Gossiper)SendRumor(rumor *message.RumorMessage, peer_to_send string) {
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
    gossiper.SendGossip(message.GossipPacket{Status: &status_packet},
                        relay_addr)
}

func (gossiper *Gossiper)SendPrivateMessage(private_msg *message.PrivateMessage, next_addr string) {
    gossiper.SendGossip(message.GossipPacket{Private: private_msg}, next_addr)
}

func (gossiper *Gossiper) WaitForAck() {
    // Waiting for Ack
    wait_map := make(map[string]map[string]chan bool)
    for {
        select {
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

func (gossiper *Gossiper) RouteRumor(){
    gossiper.RouteRumorInit()
    for {
        timer := time.NewTimer(time.Duration(RTIMER)*time.Second)
        <-timer.C
        var peer_map map[string]bool
        peer_map= gossiper.PeerList
        peer_list := []string{}
        for peer, _ := range(peer_map) { peer_list = append(peer_list, peer) }
        if len(peer_list) == 0 { continue }
        peer_to_send := peer_list[rand.Intn(len(peer_list))]
        packet := &message.RumorMessage{Origin:gossiper.Name,
                                        ID:gossiper.VectorTable[gossiper.Name],
                                        Text:""}
        gossiper.SendRumor(packet, peer_to_send)
    }
}

func (gossiper *Gossiper) RouteRumorInit(){
    for peer_to_send, _ := range(gossiper.PeerList) {
        packet := &message.RumorMessage{Origin:gossiper.Name,
                                        ID:gossiper.VectorTable[gossiper.Name],
                                        Text:""}
        gossiper.SendRumor(packet, peer_to_send)
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
    flag.Parse()

    rand.Seed(time.Now().UTC().UnixNano())
    DEBUG=false
    HOPLIMIT = 10
    MessageQueue = make(chan message.Message)
    WebServerReceiveChannel = make(chan message.ClientMessage)
    WebServerSendChannel = make(chan message.ClientMessage)
    RTIMER = *rtimer
    var peer_list []string
    if *peers == "" { peer_list = []string{}
    } else { peer_list = strings.Split(*peers, "_") }

    //fmt.Println(*ui_port, *gossipPort, *name, peer_list)
    gossiper := NewGossiper(*name, *ui_port, *gossipPort, *webport, peer_list)
    defer gossiper.Close()

    go gossiper.SendStatus()
    go gossiper.GossipPrivateMessages()
    go gossiper.WaitForAck()
    go gossiper.AntiEntropy()
    go gossiper.RouteRumor()
    gossiper.GossipMessages()
}