#!/bin/sh

set -xe

export DOCKER_HOST=tcp://localhost:2375

docker rm -f mysql-master mysql-slave || :
netctl network rm private || :
volcli volume remove policy1/mysql2 || :
volcli volume remove policy1/mysql1 || :

if [ "$(sudo rbd ls rbd | wc -l)" != "0" ]
then
  snap=$(sudo rbd snap list policy1.mysql1 | tail -n +2 | awk '{ print $2 }')
  sudo rbd snap unprotect --snap "$snap" policy1.mysql1
  sudo rbd snap purge policy1.mysql1
  sudo rbd rm policy1.mysql1
fi

netctl network create -s 172.16.24.0/24 -g 172.16.24.1 private
volcli volume create policy1/mysql1

docker run -itd --net private --name mysql-master -e MYSQL_ALLOW_EMPTY_PASSWORD=1 -v policy1/mysql1:/var/lib/mysql mysql mysqld --datadir /var/lib/mysql/databases --server-id 1 --log-bin

echo sleeping to wait for boot
sleep 60

docker exec -it mysql-master sh -c 'echo CREATE DATABASE foo | mysql -u root'
docker exec -it mysql-master sh -c "echo \"GRANT REPLICATION SLAVE ON *.* TO 'slave_user'@'%' IDENTIFIED BY 'password'; FLUSH PRIVILEGES;\" | mysql -u root" 
docker exec -it mysql-master sh -c 'echo FLUSH TABLES WITH READ LOCK | mysql -u root foo'

file=$(docker exec -it mysql-master sh -c 'echo show master status\\G | mysql -u root' | grep File | perl -ne 'chomp; @tmp = split(/:\s+/); $tmp[1] =~ s/[^\w_.-]+//g; print $tmp[1]') 
position=$(docker exec -it mysql-master sh -c 'echo show master status\\G | mysql -u root' | grep Position | perl -nle 'chomp; @tmp = split(/:\s+/); $tmp[1] =~ s/\D+//g; print $tmp[1]') 

echo $file $position

volcli volume snapshot take policy1/mysql1
sleep 1
item=$(volcli volume snapshot list policy1/mysql1 | head -1)
volcli volume snapshot copy policy1/mysql1 "$item" mysql2

docker exec -it mysql-master sh -c 'echo UNLOCK TABLES | mysql -u root foo'

docker run --rm -v policy1/mysql2:/mnt debian sh -c "sed -i -e 's/server-uuid=.*/server-uuid=$(uuidgen)/g' /mnt/databases/auto.cnf"

docker run -itd --name mysql-slave --net private -v policy1/mysql2:/var/lib/mysql mysql mysqld --datadir /var/lib/mysql/databases --server-id 2

sleep 10

docker exec -it mysql-slave sh -c "echo \"CHANGE MASTER TO MASTER_HOST='mysql-master', MASTER_USER='slave_user', MASTER_PASSWORD='password', MASTER_LOG_FILE='${file}', MASTER_LOG_POS=${position}; START SLAVE;\" | mysql -u root"

docker logs mysql-slave
