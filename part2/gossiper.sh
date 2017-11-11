#!/bin/bash
name=$@
noforward=""
if [ $name = nodepub ]; then
    noforward="-noforward"
fi
ip=$( nslookup $name | grep Address | cut -d " " -f 3)
/root/gossiper -UIPort=10001 -gossipAddr=$ip:10000 -name=$name \
  -peers=172.16.0.2:10000 -rtimer=1 $noforward 2>&1 > /root/gossip.log
