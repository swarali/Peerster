package main

import (
    "encoding/hex"
	"flag"
	"fmt"
	"net"
	"strconv"

	"github.com/dedis/protobuf"
    "github.com/Swarali/Peerster/part1/message"
)

// go run client.go -UIPort=10000 -msg=Hello
func main() {
	var ui_port = flag.Int("UIPort", 0,
                           "UI Port where the gossip program listens")
	var msg = flag.String("msg", "",
                          "Message sent from client to the server")
    var dest = flag.String("Dest", "", "Destination node for private message")
    var file = flag.String("file", "", "Name of the file to share")
    var request = flag.String("request", "", "Name of the file to share")

	flag.Parse()
	// fmt.Println(*msg, *ui_port)
	destination_addr := "127.0.0.1:" + strconv.Itoa(*ui_port)
	con, _ := net.Dial("udp", destination_addr)
	defer con.Close()

    var msg_proto *message.ClientMessage
    if *request != "" {
        hash, _ := hex.DecodeString(*request)
        msg_proto = &message.ClientMessage{ Operation: "NewFileDownload", Message: *file, Destination: *dest, HashValue: hash}
    } else if *file != "" {
        msg_proto = &message.ClientMessage{ Operation: "NewFileUpload", Message: *file, HashValue: make([]byte, 0) }
    } else if *dest != "" {
        msg_proto = &message.ClientMessage{ Operation: "NewPrivateMessage", Message: *msg, Destination: *dest , HashValue: make([]byte, 0) }
    } else {
        msg_proto = &message.ClientMessage{ Operation: "NewMessage", Message: *msg , HashValue: make([]byte, 0) }
    }
    packetBytes, proto_err :=protobuf.Encode(msg_proto)
    if proto_err != nil {
        fmt.Println("Received proto err", proto_err, "for", msg_proto)
        return
    }
	fmt.Println("Sending message", msg_proto)
	_, err := con.Write(packetBytes)
	if err != nil {
		fmt.Println(err)
	}
}
