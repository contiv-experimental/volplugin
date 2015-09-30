# -*- mode: ruby -*-
# vi: set ft=ruby :

require 'yaml'
require 'fileutils'
VAGRANTFILE_API_VERSION = '2'

config_file=File.expand_path(File.join(File.dirname(__FILE__), 'vagrant_variables.yml'))
settings=YAML.load_file(config_file)

NMONS        = settings['vms']
SUBNET       = settings['subnet']
BOX          = settings['vagrant_box']
BOX_VERSION  = settings['box_version']
MEMORY       = settings['memory']

shell_provision = <<-EOF
echo "export http_proxy='$1'" >> /etc/profile.d/envvar.sh
echo "export https_proxy='$2'" >> /etc/profile.d/envvar.sh
no_proxy='192.168.24.10,192.168.24.11,192.168.24.12,172.17.42.1,127.0.0.1,localhost'
echo "export 'no_proxy=$no_proxy'" >> /etc/profile.d/envvar.sh

. /etc/profile.d/envvar.sh

mkdir /etc/systemd/system/docker.service.d
echo "[Service]" | sudo tee -a /etc/systemd/system/docker.service.d/http-proxy.conf &>/dev/null
echo "Environment=\\\"no_proxy=$no_proxy\\\" \\\"http_proxy=$http_proxy\\\" \\\"https_proxy=$https_proxy\\\"" | sudo tee -a /etc/systemd/system/docker.service.d/http-proxy.conf &>/dev/null
sudo systemctl daemon-reload
sudo systemctl stop docker
sudo systemctl start docker
EOF

ansible_provision = proc do |ansible|
  ansible.playbook = 'ansible/site.yml'
  # Note: Can't do ranges like mon[0-2] in groups because
  # these aren't supported by Vagrant, see
  # https://github.com/mitchellh/vagrant/issues/3539
  ansible.groups = {
    'mons'        => (0..NMONS - 1).map { |j| "mon#{j}" },
  }

  proxy_env = { }

  %w[HTTP_PROXY HTTPS_PROXY http_proxy https_proxy].each do |name|
    if ENV[name]
      proxy_env[name] = ENV[name]
    end
  end

  # In a production deployment, these should be secret
  ansible.extra_vars = {
    proxy_env: proxy_env,
    fsid: '4a158d27-f750-41d5-9e7f-26ce4c9d2d45',
    monitor_secret: 'AQAWqilTCDh7CBAAawXt6kyTgLFCxSvJhTEmuw==',
    journal_size: 100,
    monitor_interface: 'enp0s8',
    cluster_network: "#{SUBNET}.0/24",
    public_network: "#{SUBNET}.0/24",
  }
  ansible.limit = 'all'
end

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
  config.vm.box = BOX
  config.vm.box_version = BOX_VERSION
  config.ssh.insert_key = false # workaround for https://github.com/mitchellh/vagrant/issues/5048
  config.vm.synced_folder ".", "/opt/golang/src/github.com/contiv/volplugin"
  config.vm.synced_folder "systemtests/testdata", "/testdata"

  (0..NMONS - 1).each do |i|
    config.vm.define "mon#{i}" do |mon|
      mon.vm.hostname = "ceph-mon#{i}"
      mon.vm.network :private_network, ip: "#{SUBNET}.1#{i}"
      mon.vm.network :private_network, ip: "#{SUBNET}.10#{i}"
      #mon.vm.network :private_network, ip: "#{SUBNET}.20#{i}"
      mon.vm.provider :virtualbox do |vb|
        (0..1).each do |d|
          disk_path = "disk-#{i}-#{d}"
          vdi_disk_path = disk_path + ".vdi"

          vb.customize ['createhd',
                        '--filename', disk_path,
                        '--size', '11000']
          # Controller names are dependent on the VM being built.
          # It is set when the base box is made in our case ubuntu/trusty64.
          # Be careful while changing the box.
          vb.customize ['storageattach', :id,
                        '--storagectl', 'SATA Controller',
                        '--port', 3 + d,
                        '--type', 'hdd',
                        '--medium', vdi_disk_path]
        end

        vb.customize ['modifyvm', :id, '--memory', "#{MEMORY}"]
      end

      mon.vm.provider :vmware_fusion do |v|
        v.vmx['memsize'] = "#{MEMORY}"
      end

      mon.vm.provision "shell" do |s|
        s.inline = shell_provision
        s.args = [ ENV["http_proxy"] || "", ENV["https_proxy"] || "" ]
      end

      # Run the provisioner after the last machine comes up
      config.vm.provision 'ansible', &ansible_provision if i == (NMONS - 1)
    end
  end
end
