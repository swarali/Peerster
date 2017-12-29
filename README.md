Run the gossiper with follwing command:
go run gossiper.go -UIPort=10000 -gossipAddr=127.0.0.1:5000 -name=A -peers=127.0.0.1:5001
go run gossiper.go -UIPort=10001 -gossipAddr=127.0.0.1:5001 -name=B -peers=127.0.0.1:5000 -webport=8081

Run the client with follwing command:
go run client/client.go -UIPort=10000 -file=a.txt
go run client/client.go -UIPort=10002 -keywords=AA
go run client/client.go -UIPort=10000 -file=ABAB.txt -request=56827f4ac29c8f767a29bdb3d300053cb6823d68375bba3eb28d61b9d6480d98

The files should be uploaded from the files directory.
Included tests files - test.sh ans test1.sh
