go run gossiper.go -UIPort=10000 -gossipAddr=127.0.0.1:5000 -name=Swarali_5_0 -peers=127.0.0.1:5001 -rtimer=20 > a.log &

go run gossiper.go -UIPort=10001 -gossipAddr=127.0.0.1:5001 -name=Swarali_5_1 -peers=127.0.0.1:5000 -webport=8081 -rtimer=20 >b.log &

sleep 5
go run client/client.go -UIPort=10000 -file=a.txt
go run client/client.go -UIPort=10001 -file=b.txt

sleep 5
go run client/client.go -UIPort=10001 -file=a.txt -Dest=Swarali_5_0 -request=b59bd2b4f08d360aebf5fbfb6881daebf13c73079a67020ea2b5b6627724bfa9

sleep 30
ls -al 127*
pkill -f gossiper
