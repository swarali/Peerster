Some changes required for the tests to run properly
- Cannot get name from hostname. So need to run gossiper.sh as >./gossiper.sh nodea [Included in part2/]
- client cannot run without -UIPort flag.Change included in the test_lib.sh file [Included in part2]
- Need to update iptables after starting docker containers. update iptables part in testscript [Included in part2/test_libs.sh]

For tests:
- test test_hw2_ex1.sh needs a higher timeout. Please change sleep to 5 sec instead of 1 sec. [Not included in the package]

Organisation of files:
-$GOPATH/src/github.com/Swarali/Peerster
    -part1
        -gossiper.go & other helper files
        -client
            -client.go
    -part2
        -gossiper.go & other helper files
        -client
            -client.go
        -Modified gossiper.sh & test_lib.sh to run the tests successfully
