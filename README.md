Run the gossiper with follwing command:
go run gossiper.go -UIPort=10000 -gossipAddr=127.0.0.1:5000 -name=Swarali_5_0 -peers=127.0.0.1:5001
go run gossiper.go -UIPort=10001 -gossipAddr=127.0.0.1:5001 -name=Swarali_5_1 -peers=127.0.0.1:5000 -webport=8081

Run the client with follwing command:
go run client/client.go -UIPort=10000 -file=a.txt
go run client/client.go -UIPort=10002 -keywords=AA
go run client/client.go -UIPort=10000 -file=ABAB.txt -request=56827f4ac29c8f767a29bdb3d300053cb6823d68375bba3eb28d61b9d6480d98

The files should be uploaded from the files directory of the respective parts - part1 & part2. Gossiper will crash if it cannot 
find the path.
Included tests files - test.sh in part1 and part2
