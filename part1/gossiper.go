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
    "github.com/JohnDoe/Peerster/part3/message"
    "github.com/JohnDoe/Peerster/part3/webserver"
    )

// go run gossiper.go -UIPort=10000 -gossipPort=5000 -name=nodeA -peers=127.0.0.1:5001_10.1.1.7:5002
var DEBUG bool
var MessageQueue chan message.Message

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
    VectorTable map[string]uint32
    Messages map[string][]string
    PeerList map[string]bool
    AckReceiveChannel chan string
    AckSendChannel chan AckWait
}

func NewGossiper(name, ui_port, gossip_port string,
                 peer_list []string) *Gossiper {

    peer_list_map := make(map[string]bool)
    for _, peer := range peer_list { peer_list_map[peer] = true }
    rumor_channel := make(chan message.GossipMessage)
    status_channel := make(chan message.GossipMessage)
    client_conn := ListenOn("127.0.0.1:" + ui_port)

    gossip_conn := ListenOn(gossip_port)

    //vectorTable := map[string]int{name:1}
    gossiper:= &Gossiper{Name: name,
                          UIPort: ui_port,
                          UIConn: client_conn,
                          GossipPort: gossip_port,
                          GossipConn: gossip_conn,
                          RumorChannel: rumor_channel,
                          StatusChannel: status_channel,
                          VectorTable: make(map[string]uint32),
                          Messages: make(map[string][]string),
                          PeerList: peer_list_map,
                          AckReceiveChannel: make(chan string),
                          AckSendChannel: make(chan AckWait),
                        }
    if DEBUG { fmt.Println("Start receiving on", ui_port) }
    go ReceiveClientMessage(client_conn)

    if DEBUG { fmt.Println("Start receiving on", gossip_port) }
    go ReceiveGossipMessage(gossip_conn)

    go gossiper.ProcessMessageQueue()
    return gossiper
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
        MessageQueue<-message.Message{ClientMsg: packet}
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
            } else {
                channel = gossiper.StatusChannel
            }
        } else {
            if msg.ClientMsg.ID != "" {
                gossiper.Name = msg.ClientMsg.ID
                continue
            } else if msg.ClientMsg.Peer != "" {
                gossiper.UpdatePeer(msg.ClientMsg.Peer)
                continue
            } else {
                gossip_packet = message.GossipPacket{}
                gossip_packet.Rumor = &message.RumorMessage{}
                gossip_packet.Rumor.Origin = gossiper.Name
                gossip_packet.Rumor.Message.Text = msg.ClientMsg.Text
                relay_addr = "N/A"
                channel = gossiper.RumorChannel
            }
        }
        gossiper.UpdatePeer(relay_addr)
        gossiper.PrintPacket(gossip_packet, relay_addr)
        gossiper.PrintPeers()
        channel<-message.GossipMessage{Packet:gossip_packet,
                                       Relay_addr:relay_addr}
    }
}

func (gossiper *Gossiper)PrintPacket(packet message.GossipPacket, relay_addr string) {
    if packet.Rumor != nil {
        if relay_addr == "N/A" { fmt.Printf("CLIENT %s %s\n", packet.Rumor.Message.Text, gossiper.Name)
        } else { fmt.Printf("RUMOR origin %s from %s ID %d contents %s\n", packet.Rumor.Origin, relay_addr, packet.Rumor.Message.ID,
                            packet.Rumor.Message.Text)
        }
    } else {
        status_str := fmt.Sprintf("STATUS from %s", relay_addr)
        for _, peer_status := range packet.Status.Want {
            peer_status_str := fmt.Sprintf("origin %s nextID %d", peer_status.Identifier, peer_status.NextID)
            status_str = fmt.Sprintf("%s %s", status_str, peer_status_str)
        }
        fmt.Println(status_str)
    }
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
    }
}

func (gossiper *Gossiper)SendAck(relay_addr string) {
    status_packet := message.StatusPacket{}
    for origin, val := range gossiper.VectorTable {
        peer_status := message.PeerStatus {Identifier: origin,
                                           NextID:val}
        status_packet.Want = append(status_packet.Want, peer_status)
    }
    // Send Ack
    packet := message.GossipPacket{ Status: &status_packet}
    packetBytes, _ := protobuf.Encode(&packet)
    udpAddr, _ := net.ResolveUDPAddr("udp", relay_addr)
    gossiper.GossipConn.WriteToUDP(packetBytes, udpAddr)
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
                peer_msg := message.PeerMessage{ID: peer_next_id,
                                               Text: gossiper.Messages[origin][peer_next_id-1]}
                rumor_msg = &message.RumorMessage{Origin: origin,
                                                  Message: peer_msg}
                break
            }
        }
        if gossiper_has_new_msg {
            packet := message.GossipPacket{ Rumor: rumor_msg}
            go gossiper.SendRumor(packet, relay_addr)
            //continue
        }

        remote_has_new_msg := false
        for origin, peer_next_id := range peer_vector_map {
            next_id, ok := gossiper.VectorTable[origin]
            if ok != true || next_id < peer_next_id {
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
            packet.Message.ID = next_id
            //packet.Message.ID = gossiper.VectorTable[packet.Origin]
        }

        next_id, ok := gossiper.VectorTable[packet.Origin]
        if !ok {
            //gossiper.Messages[packet.Origin] = make(string[])
            gossiper.VectorTable[packet.Origin] = 1
            next_id = 1
        }

        if packet.Message.ID == next_id {
            message_list := append(gossiper.Messages[packet.Origin],
                                   packet.Message.Text)
            gossiper.Messages[packet.Origin] = message_list
            gossiper.VectorTable[packet.Origin] += 1

            if DEBUG {gossiper.PrintPeers()
                      fmt.Println("Messages:", gossiper.Messages,
                        "Vector Table:", gossiper.VectorTable)}

            go gossiper.TransmitMessage(channel_packet,
                                        gossiper.PeerList)
        } else {
            //Discard the message since it is not in order
            if DEBUG {
                fmt.Println("Discarding the message")
                gossiper.PrintPeers()
                fmt.Println("Messages:", gossiper.Messages,
                            "Vector Table:", gossiper.VectorTable)
            }
        }

        if relay_addr != "N/A" { go gossiper.SendAck(relay_addr)}
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
        gossiper.SendRumor(packet, peer_to_send)

        if (rand.Int() % 2) == 0 { break }
        flipped_coin = true
    }
}

func (gossiper *Gossiper)SendRumor(packet message.GossipPacket,
                                   peer_to_send string) {
    for {
        if DEBUG { fmt.Println("Peer to send is ", peer_to_send) }
        packetBytes, _ := protobuf.Encode(&packet)
        udpAddr, _ := net.ResolveUDPAddr("udp", peer_to_send)
        gossiper.GossipConn.WriteToUDP(packetBytes, udpAddr)

        // Wait for ack or timeout
        timer := time.NewTimer(time.Second)
        ack_wait := make(chan bool, 1)
        ack_id := packet.Rumor.Origin + ":" + fmt.Sprint(packet.Rumor.Message.ID)
        gossiper.AckSendChannel<- AckWait{Addr: peer_to_send,
                                          Ack_id : ack_id,
                                          Ack_wait: ack_wait}
        select {
            case <-ack_wait:
                // Stop rumormongering
                if DEBUG { fmt.Println("Ack received") }
                return
            case <-timer.C:
                if DEBUG { fmt.Println("Timeout") }
                // On timeout resend the packet 
        }
    }
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
                wait_map[addr][ack_id] = ack_wait
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
            for peer, _ := range(peer_map) { peer_list = append(peer_list, peer) }
            peer_to_send := peer_list[rand.Intn(len(peer_list))]
            gossiper.SendAck(peer_to_send)
        }
}

func (gossiper *Gossiper) StartWebServer() {

    get_channel := make(chan chan bool)
    post_channel := make(chan [2]string)
    index_page := &webserver.Page{}
    go webserver.StartWebServer(get_channel, post_channel, index_page)
    for {
        select {
            case get_request := <-get_channel:
                var peer_list  map[string]bool
                peer_list= gossiper.PeerList
                webserver.IndexPage = &webserver.Page{Name:gossiper.Name, Peers: peer_list, Messages: gossiper.Messages}
                get_request<-true
            case post_request := <-post_channel:
                client_msg := message.ClientMessage{}
                key := post_request[0]
                val := post_request[1]
                if key == "ID" { client_msg.ID = val
                } else if key == "Text" {client_msg.Text = val
                } else { client_msg.Peer = val }
                MessageQueue<-message.Message{ClientMsg:&client_msg}
        }
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
    var gossipPort = flag.String("gossipPort", "",
                    "Address for the current gossiper")
    var name = flag.String("name", "", "Name of the current host")
    var peers = flag.String("peers", "",
                    "List of peers in the form <ip>:port separated by an underscore")
    flag.Parse()

    DEBUG=false
    rand.Seed(time.Now().UTC().UnixNano())
    MessageQueue = make(chan message.Message)
    var peer_list []string
    if *peers == "" { peer_list = []string{}
    } else { peer_list = strings.Split(*peers, "_") }

    //fmt.Println(*ui_port, *gossipPort, *name, peer_list)
    gossiper := NewGossiper(*name, *ui_port, *gossipPort, peer_list)
    defer gossiper.Close()

    go gossiper.SendStatus()
    go gossiper.WaitForAck()
    go gossiper.AntiEntropy()
    messages := make(map[string][]string)
    messages["A"] = []string{"aaa", "deeqerq"}
    messages["B"] = []string{"hhhh", "qwrqrq"}
    //messages := map[string][]string{"A":["aaaa","aa"]}
    go gossiper.StartWebServer()
    gossiper.GossipMessages()
}
