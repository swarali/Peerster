#A
go run gossiper.go -UIPort=10000 -gossipAddr=127.0.0.1:5000 -name=A -peers=127.0.0.1:5001_127.0.0.1:5002 -rtimer=20 > /dev/null &

#B
go run gossiper.go -UIPort=10001 -gossipAddr=127.0.0.1:5001 -name=B -peers=127.0.0.1:5000_127.0.0.1:5002 -webport=8081 -rtimer=20 > /dev/null &

#C
go run gossiper.go -UIPort=10002 -gossipAddr=127.0.0.1:5002 -name=C -peers=127.0.0.1:5000_127.0.0.1:5001_127.0.0.1:5003_127.0.0.1:5004 -webport=8082 -rtimer=20 > /dev/null &

#D
go run gossiper.go -UIPort=10003 -gossipAddr=127.0.0.1:5003 -name=D -peers=127.0.0.1:5002_127.0.0.1:5005 -webport=8083 -rtimer=20 > /dev/null &

#E
go run gossiper.go -UIPort=10004 -gossipAddr=127.0.0.1:5004 -name=E -peers=127.0.0.1:5002_127.0.0.1:5005 -webport=8084 -rtimer=20 > /dev/null &

#F
go run gossiper.go -UIPort=10005 -gossipAddr=127.0.0.1:5005 -name=F -peers=127.0.0.1:5003_127.0.0.1:5004 -webport=8085 -rtimer=20 > /dev/null &
