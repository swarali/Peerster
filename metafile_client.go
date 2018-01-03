package main

import (
	"fmt"
	"os"
	"log"
	"io"
	"net"
)

const BUFFERSIZE = 1024

func main() {
	sendFileClient("127.0.0.1:5000")
}

func sendFileClient(address string) {
	conn, err := net.Dial("tcp", address)
	sendBuffer := make([]byte, BUFFERSIZE)

	if err != nil {
		panic(err)
	}

	defer conn.Close()

	fmt.Println("Connected to server")

	file, err := os.Open("Swarali_5_0.log")

	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	for {
		n, err := file.Read(sendBuffer)

		if n > 0 {
			fmt.Println(string(sendBuffer[:n])) // your read buffer.
			conn.Write(sendBuffer[:n])
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("read %d bytes: %v", n, err)
			break
		}
	}

	fmt.Println("Sent File")
}
