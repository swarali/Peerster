# Peerster: File Streaming Between Trusted Peers
By Ariel Alba, Swarali Karkhanis

This project proposes Peerster, a P2P secure file streaming system developed as a part of Decentralised Systems Engineering in Autumn'17. Peerster system allows nodes to stream data securely from its trusted peers in an unsecure environment. The properties of the system are as follows:
- Security
  - Confidentiality: Through trusted relays 
  - Authentication: Through packet signatures
  - Integrity: Through packet signatures and metadata hashing

- Streaming
  - Chunking and Hashing of Metadata file
  - Encoding and Chunking of Video files

The high-level design of Peerster is as follows:
- A new node on the network shares its public key across all the peers. Based on their own preferance every node decides whether or not to trust a node.
- Nodes share the file metadata on the network by using gossiping and anti-entropy protocols.
- A node can issue a search request (using key-words) and receive files(matching the keyword) and list of nodes from which files can be downloaded.
- A node can issue a download request (using metadata hash) on trusted nodes by requesting on chunks of data from different nodes on the network. A node need not have the entire file only few chunks are enough.
- A node replies to the download request only if it trusts the requesting node. To maintain integrity and confidentiality, we use PGP encryption/decryption for all data/metadata related communications.

We have also included a web interface that supports viewing of the streamed video files.

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
