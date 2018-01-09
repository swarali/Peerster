go run gossiper.go -UIPort=10000 -gossipAddr=127.0.0.1:5000 -name=Swarali_5_0 -peers=127.0.0.1:5001 -rtimer=20 > /dev/null &

go run gossiper.go -UIPort=10001 -gossipAddr=127.0.0.1:5001 -name=Swarali_5_1 -peers=127.0.0.1:5000 -webport=8081 -rtimer=20 > /dev/null &

go run gossiper.go -UIPort=10002 -gossipAddr=127.0.0.1:5002 -name=Swarali_5_2 -peers=127.0.0.1:5001 -webport=8082 -rtimer=20 > /dev/null &

go run gossiper.go -UIPort=10003 -gossipAddr=127.0.0.1:5001 -name=Swarali_5_3 -peers=127.0.0.1:5002 -webport=8083 -rtimer=20 > /dev/null &

go run gossiper.go -UIPort=10004 -gossipAddr=127.0.0.1:5002 -name=Swarali_5_4 -peers=127.0.0.1:5003 -webport=8084 -rtimer=20 > /dev/null &

sleep 40

go run client/client.go -UIPort=10000 -file=vid2.mp4720p.webm

sleep 5

go run client/client.go -UIPort=10001 -keywords=vid

sleep 20

go run client/client.go -UIPort=10001 -file=vid2.mp4720p.webm -request=6fc3d4d1566435224c8470f2c4e7b9ed0928b938b419e5fb3f3738e3f63e0538

sleep 20
ls -al 127*
pkill -f gossiper

