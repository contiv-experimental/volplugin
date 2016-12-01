#!/bin/bash

set -e

# contact swarm instead of just docker
export DOCKER_HOST=tcp://localhost:2375

NODES=4 # must at least be 4

# clean up from prior runs
for i in $(seq 1 $NODES)
do
  docker rm -f cassandra${i} || :
  volcli volume remove policy1/cassandra${i} || :
  volcli volume create policy1/cassandra${i} || :
done

netctl network rm private || :
netctl network create -s 172.16.24.0/24 -g 172.16.24.1 private

# start our first cass node; this will be used as the seed for others.
docker run --name cassandra1 -itd --net private -v policy1/cassandra1:/var/lib/cassandra cassandra
echo waiting 60s for bootstrap node
sleep 60

# start the rest of the nodes. note that nodes 2-3 will also be used as seeds
# when they come up. This is consistent with cassandra operational guidelines.
for i in $(seq 2 $NODES)
do
  docker run --name cassandra$i -itd --net private -v policy1/cassandra$i:/var/lib/cassandra -e CASSANDRA_SEEDS=cassandra1,cassaandra2,cassandra3 cassandra
  echo waiting 45s to avoid bootstrap conflicts
  sleep 45
done

echo waiting 60s for instances to settle
sleep 60

#
# prove that we did it!
#
docker exec -it cassandra1 nodetool status
docker ps -f name=cassandra
