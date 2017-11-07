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
	flag.Parse()
	// fmt.Println(*msg, *ui_port)
	destination_addr := "127.0.0.1:" + strconv.Itoa(*ui_port)
	con, _ := net.Dial("udp", destination_addr)
	defer con.Close()

    msg_proto := &message.ClientMessage{ Text: *msg }
	//msg_proto := &message.Message{ Origin_name: "N/A",
    //                               Text: *msg}
	packetBytes,_ :=protobuf.Encode(msg_proto)

	fmt.Println("Sending message", msg_proto.Text)
	_, err := con.Write(packetBytes)
	if err != nil {
		fmt.Println(err)
	}
	// for i := 0; i<10; i++ {
		// fmt.Println("Sending message ", i)
		// message := fmt.Sprintf("%s %d", *msg, i)
		// message = "Hello"
		// write_buf := []byte(message)
		// _, err := con.Write(write_buf)
		// if err != nil {
			// fmt.Println(err)
		// }
	// }

}
