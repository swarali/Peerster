package main

import (
	"os"
	"fmt"
	"net"
	"bufio"
	"log"
)

const BUFFERSIZE = 1024

func main() {
	tcp_conn := ListenTCPOn2("127.0.0.1:5000")
	ReceiveTCPMessage2(tcp_conn)
}

func ListenTCPOn2(address string) *net.TCPListener {
	tcpAddr, _ := net.ResolveTCPAddr("tcp", address)
	tcpConn, _ := net.ListenTCP("tcp", tcpAddr)
	return tcpConn
}

func ReceiveTCPMessage2(listener *net.TCPListener) {
	for {
		connection, err := listener.Accept()

		if err != nil {
			fmt.Println("Error: ", err)
			os.Exit(1)
		}

		fmt.Println("Client connected")
		go sendFileToClient2(connection)
	}
}

func sendFileToClient2(connection net.Conn) {
	fmt.Println("A client has connected!")

	defer connection.Close()

	newFile, err := os.Create("Swarali_5_1copy.log")

	if err != nil {
		panic(err)
	}

	defer newFile.Close()

	for {
		connbuf := bufio.NewReader(connection)
		// Read the first byte and set the underlying buffer
		b, _ := connbuf.ReadByte()
		if connbuf.Buffered() > 0 {
			var msgData []byte
			msgData = append(msgData, b)
			for connbuf.Buffered() > 0 {
				// read byte by byte until the buffered data is not empty
				b, err := connbuf.ReadByte()
				if err == nil {
					msgData = append(msgData, b)
				} else {
					log.Println("-------> unreadable caracter...", b)
				}
			}

			newFile.Write(msgData)
		}
	}
}


