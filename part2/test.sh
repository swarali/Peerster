go run gossiper.go -UIPort=10000 -gossipAddr=127.0.0.1:5000 -name=Swarali_5_0 -peers=127.0.0.1:5001 -rtimer=20 > a.log &

go run gossiper.go -UIPort=10001 -gossipAddr=127.0.0.1:5001 -name=Swarali_5_1 -peers=127.0.0.1:5000 -webport=8081 -rtimer=20 >b.log &

go run gossiper.go -UIPort=10002 -gossipAddr=127.0.0.1:5002 -name=Swarali_5_2 -peers=127.0.0.1:5001 -webport=8082 -rtimer=20 >c.log &
sleep 40

#go run client/client.go -UIPort=10000 -msg=A
#go run client/client.go -UIPort=10001 -msg=B
#go run client/client.go -UIPort=10002 -msg=C
#go run client/client.go -UIPort=10000 -msg=D
go run client/client.go -UIPort=10000 -file=AAAA.txt
go run client/client.go -UIPort=10001 -file=ABAB.txt

sleep 5

go run client/client.go -UIPort=10002 -keywords=AA,AB
go run client/client.go -UIPort=10000 -keywords=AB
go run client/client.go -UIPort=10001 -keywords=A
sleep 20

go run client/client.go -UIPort=10000 -file=ABAB.txt -request=56827f4ac29c8f767a29bdb3d300053cb6823d68375bba3eb28d61b9d6480d98
go run client/client.go -UIPort=10002 -file=AAAA.txt -request=b59bd2b4f08d360aebf5fbfb6881daebf13c73079a67020ea2b5b6627724bfa9
#go run client/client.go -UIPort=10001 -file=a.txt -Dest=Swarali_5_0 -request=b59bd2b4f08d360aebf5fbfb6881daebf13c73079a67020ea2b5b6627724bfa9

sleep 20
ls -al 127*
pkill -f gossiper

