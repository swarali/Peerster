package message

type PeerMessage struct {
    ID uint32
    Text string
}

type RumorMessage struct {
    Origin string
    Message PeerMessage
}

type PeerStatus struct {
    Identifier string
    NextID uint32
}

type StatusPacket struct {
    Want []PeerStatus
}

type GossipPacket struct {
    Rumor *RumorMessage
    Status *StatusPacket
}

type GossipMessage struct {
    Packet GossipPacket
    Relay_addr string
}

type ClientMessage struct {
    ID, Text, Peer string
}
type Message struct {
    GossipMsg *GossipMessage
    ClientMsg *ClientMessage
}
