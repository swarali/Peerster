# Machine Learning Project II

In recent years, decentralized systems, especially peer-to-peer systems have gained immense popularity in the sphere of content distribution. 
Concepts like Streaming, File Sharing, Cloud storage, Cloud computing, are not taboo concepts for people anymore. 
Those services are changing the way we live and creating new necessities to fulfill. 
Its advantages like speed, scalability, dynamic node additions/deletions make it a very attractive choice for file distribution over the traditional client-server model.
The evolution of systems from the age of Napster to BitTorrent, Gnutella is the testimony to the fact that the internet sytems are moving towards P2P file sharing approach.
This can also be seen in the rise of several serverless CDN solutions that try to leverage technologies like WebRTC for peer communication. 

## Prerequisites

Run commands 

```
go get golang.org/x/crypto/openpgp
go get golang.org/x/crypto/openpgp/armor
go get golang.org/x/crypto/openpgp/packet
```

Install Homebrew https://brew.sh

```
/usr/bin/ruby -e "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)"
```

Install ffmpeg

```
brew install ffmpeg
```

## Running the projects

Run the gossiper with follwing command:
go run gossiper.go -UIPort=10000 -gossipAddr=127.0.0.1:5000 -name=A -peers=127.0.0.1:5001
go run gossiper.go -UIPort=10001 -gossipAddr=127.0.0.1:5001 -name=B -peers=127.0.0.1:5000 -webport=8081

Run the client with follwing command:
go run client/client.go -UIPort=10000 -file=a.txt
go run client/client.go -UIPort=10002 -keywords=AA
go run client/client.go -UIPort=10000 -file=ABAB.txt -request=56827f4ac29c8f767a29bdb3d300053cb6823d68375bba3eb28d61b9d6480d98

The files should be uploaded from the files directory.

Included tests files - test.sh, test1.sh and test_streaming.sh
