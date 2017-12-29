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

type DataRequest struct {
    Origin string
    Destination string
    HopLimit uint32
    FileName string
    HashValue []byte
}

type DataReply struct {
    Origin string
    Destination string
    HopLimit uint32
    FileName string
    HashValue []byte
    Data []byte
}

type SearchRequest  struct {
    Origin    string
    Budget    uint64
    Keywords  []string
}

type SearchResult struct {
    FileName string
    MetafileHash []byte
    ChunkMap []uint64
}

type SearchReply struct {
    Origin string
    Destination string
    HopLimit uint32
    Results []*SearchResult
}

type GossipPacket struct {
    Rumor *RumorMessage
    Status *StatusPacket
    Private *PrivateMessage
    Request *DataRequest
    Reply *DataReply
    SRequest *SearchRequest
    SReply *SearchReply
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
    HashValue []byte
    Budget int
}
type Message struct {
    GossipMsg *GossipMessage
    ClientMsg *ClientMessage
}
