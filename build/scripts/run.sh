#!/bin/bash

set -e

host=$(hostname)


clean_container () {
	if docker ps -a | grep $1 -q; then
		docker rm -fv $1
	fi
}

wait_for_etcd () {
	if ! etcdctl cluster-health | grep "cluster is healthy" -q; then
		echo $host, " waiting for etcd cluster state to be healthy.."
		sleep 1
	fi
}

if [ ! -f /etc/systemd/system/volplugin.service ]; then
	sudo cp ./build/scripts/volplugin.service /etc/systemd/system/
	sudo cp ./build/scripts/volsupervisor.service /etc/systemd/system/
	sudo cp ./build/scripts/apiserver.service /etc/systemd/system/
	sudo cp ./build/scripts/volplugin.sh /usr/bin/
	sudo cp ./build/scripts/volsupervisor.sh /usr/bin/
	sudo cp ./build/scripts/apiserver.sh /usr/bin/
	sudo cp ./build/scripts/contiv-vol-run.sh /usr/bin/
	sudo systemctl daemon-reload
	wait_for_etcd
fi

# Include the dependencies
./build/scripts/deps.sh

clean_container apiserver
clean_container volplugin
clean_container volsupervisor

fast=${1:-false}
if $fast; then
	# Inputs expected
	localregistrypath=$2
	localregistryip=$3
	if [ $host == "mon0" ]; then
		# Registry container is run only on first node
		if ! docker ps | grep localregistry -q; then
			# Container is not running. clean if there is a stopped registry container
			clean_container localregistry
			docker run -d -p 5000:5000 --restart=always --name localregistry registry
		fi
	fi

	# Add a host entry for contiv-reg in /etc/hosts if it does not exist
	if ! sudo grep contiv-reg /etc/hosts -q; then
		echo $host, " adding a host entry for ", ${localregistryip}
		echo ${localregistryip} contiv-reg | sudo tee --append /etc/hosts
	fi

	# Ensure that docker allows our insecure registry if not already allowed
	if ! sudo grep insecure-registry /usr/lib/systemd/system/docker.service -q; then
		echo $host, " enabled insecure-registry option in docker.."
		sudo sed -i 's/ExecStart=.*/& --insecure-registry=contiv-reg:5000/g' /usr/lib/systemd/system/docker.service
		sudo systemctl daemon-reload
		sudo systemctl restart docker
		wait_for_etcd
	fi


	if [ $host == "mon0" ]; then
		# Create and push the volplugin image to our private docker registry
		docker build -t contiv/volplugin .
		docker tag contiv/volplugin ${localregistrypath}contiv/volplugin
		docker push ${localregistrypath}contiv/volplugin
	fi

	if [ $host != "mon0" ]; then
		# This image is already available on mon0.
		# Execute for all other hosts
		docker pull ${localregistrypath}contiv/volplugin
	fi

else
	docker build -t contiv/volplugin .
fi

# Ensure that docker is running with MountFlags=shared
if ! sudo grep "MountFlags" /usr/lib/systemd/system/docker.service | grep "shared" -q; then
	echo $host, "setting MountFlags=shared and restarting docker..."
	sudo sed -i 's/MountFlags=slave/MountFlags=shared/g' /usr/lib/systemd/system/docker.service
	sudo systemctl daemon-reload
	sudo systemctl restart docker
	wait_for_etcd
fi

echo $host " starting containers..."
sudo systemctl restart volplugin apiserver

if [ $host == "mon0" ]; then
	sudo systemctl restart volsupervisor

	# Wait for the server to be available
	connwait 127.0.0.1:9005
	volcli global upload < /testdata/globals/global1.json
fi

# Remove any leftover images
count=$(docker images -f "dangling=true" -q | wc -l)
if [ $count -gt 0 ]; then
	docker rmi $(docker images -f "dangling=true" -q)
fi
