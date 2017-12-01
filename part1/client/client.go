package main

import (
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

	flag.Parse()
	// fmt.Println(*msg, *ui_port)
	destination_addr := "127.0.0.1:" + strconv.Itoa(*ui_port)
	con, _ := net.Dial("udp", destination_addr)
	defer con.Close()

    var msg_proto *message.ClientMessage
    if *dest != "" {
        msg_proto = &message.ClientMessage{ Operation: "NewPrivateMessage", Message: *msg, Destination: *dest }
    } else if *file != "" {
        msg_proto = &message.ClientMessage{ Operation: "NewFileUpload", Message: *file}
    } else {
        msg_proto = &message.ClientMessage{ Operation: "NewMessage", Message: *msg }
    }
    packetBytes,_ :=protobuf.Encode(msg_proto)

	fmt.Println("Sending message", msg_proto.Message)
	_, err := con.Write(packetBytes)
	if err != nil {
		fmt.Println(err)
	}
}
