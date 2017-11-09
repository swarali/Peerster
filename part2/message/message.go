package message

import (
    "net"
    )
type RumorMessage struct {
    Origin string
    ID uint32
    Text string
    LastIP *net.IP
    LastPort *int
}

type PeerStatus struct {
    Identifier string
    NextID uint32
}

type StatusPacket struct {
    Want []PeerStatus
}

type PrivateMessage struct {
    Origin      string
    ID          uint32
    Text        string
    Destination string
    HopLimit    uint32
}

type GossipPacket struct {
    Rumor *RumorMessage
    Status *StatusPacket
    Private *PrivateMessage
}

type GossipMessage struct {
    Packet GossipPacket
    Relay_addr string
}

type ClientMessage struct {
    Operation string
    Message string
    Destination string
    Origin string
}
type Message struct {
    GossipMsg *GossipMessage
    ClientMsg *ClientMessage
}
