# -*- mode: ruby -*-
# vi: set ft=ruby :

require 'yaml'
require 'fileutils'
VAGRANTFILE_API_VERSION = '2'
OSX_VMWARE_DIR = "/Applications/VMware Fusion.app/Contents/Library/"

config_file=File.expand_path(File.join(File.dirname(__FILE__), 'vagrant_variables.yml'))
settings=YAML.load_file(config_file)

if ENV["DEMO"]
  settings["vms"] = 1
  settings["memory"] = 2048
end

NMONS        = ENV["VMS"] || settings['vms']
SUBNET       = settings['subnet']
BOX          = settings['vagrant_box']
BOX_VERSION  = settings['box_version']
memory       = settings['memory']

if ENV["BIG"]
  memory = 8192
end

MEMORY   = memory
NO_PROXY = '192.168.24.50,192.168.24.10,192.168.24.11,192.168.24.12'

shell_provision = <<-EOF
echo "export http_proxy='$1'" >> /etc/profile.d/envvar.sh
echo "export https_proxy='$2'" >> /etc/profile.d/envvar.sh
echo "export no_proxy='#{NO_PROXY}'" >> /etc/profile.d/envvar.sh

. /etc/profile.d/envvar.sh
EOF

ansible_provision = proc do |ansible|
  ansible.playbook = 'ansible/site.yml'
  # Note: Can't do ranges like mon[0-2] in groups because
  # these aren't supported by Vagrant, see
  # https://github.com/mitchellh/vagrant/issues/3539
  ansible.groups = {
    'volplugin-test' => (0..NMONS - 1).map { |j| "mon#{j}" },
  }

  proxy_env = {
    "no_proxy" => NO_PROXY
  }

  %w[HTTP_PROXY HTTPS_PROXY http_proxy https_proxy].each do |name|
    if ENV[name]
      proxy_env[name] = ENV[name]
    end
  end

  # In a production deployment, these should be secret
  ansible.extra_vars = {
    docker_version: "1.11.1",
    use_nfs: true,
    swarm_bootstrap_node_name: "mon0",
    docker_device: "/dev/sdb",
    etcd_peers_group: 'volplugin-test',
    env: proxy_env,
    fsid: '4a158d27-f750-41d5-9e7f-26ce4c9d2d45',
    monitor_secret: 'AQAWqilTCDh7CBAAawXt6kyTgLFCxSvJhTEmuw==',
    journal_size: 100,
    control_interface: "enp0s8",
    netplugin_if: "enp0s9",
    cluster_network: "#{SUBNET}.0/24",
    public_network: "#{SUBNET}.0/24",
    devices: "[ '/dev/sdc', '/dev/sdd' ]",
    service_vip: "192.168.24.50",
    journal_collocation: 'true',
    validate_certs: 'no',
  }
  ansible.limit = 'all'
end

def create_vmdk(name, size)
  dir = Pathname.new(__FILE__).expand_path.dirname
  path = File.join(dir, '.vagrant', name + '.vmdk')
  command = "vmware-vdiskmanager"
  args = "-c -s #{size} -t 0 -a scsi #{path} 2>&1 >/dev/null"

  if Dir.exist?(OSX_VMWARE_DIR)
    command = "'#{OSX_VMWARE_DIR}/vmware-vdiskmanager'"
  end

  %x[#{command} #{args}] unless File.exist?(path)
  return path
end

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
  config.vm.box = BOX
  config.vm.box_version = BOX_VERSION

  config.vm.synced_folder ".", "/opt/golang/src/github.com/contiv/volplugin"
  config.vm.synced_folder "systemtests/testdata", "/testdata"
  config.vm.synced_folder "bin", "/tmp/bin"

  (0..NMONS-1).each do |i|
    config.vm.define "mon#{i}" do |mon|
      mon.vm.hostname = "mon#{i}"

      [:vmware_desktop, :vmware_workstation, :vmware_fusion].each do |provider|
        mon.vm.provider provider do |v, override|
          override.vm.network :private_network, type: "dhcp", ip: "#{SUBNET}.1#{i}", auto_config: false
          override.vm.network :private_network, type: "dhcp", ip: "#{SUBNET}.2#{i}", auto_config: false
          v.vmx["scsi0:1.present"] = 'TRUE'
          v.vmx["scsi0:1.fileName"] = create_vmdk("docker-#{i}", '11000MB')

          (1..2).each do |d|
            v.vmx["scsi0:#{d + 1}.present"] = 'TRUE'
            v.vmx["scsi0:#{d + 1}.fileName"] = create_vmdk("disk-#{i}-#{d}", '11000MB')
          end

          v.vmx['memsize'] = "#{MEMORY}"

          override.vm.provision 'shell' do |s|
            s.inline = <<-EOF
              #{shell_provision}
              if sudo ip link | grep -q ens33
              then
                sudo ip link set dev ens33 down
                sudo ip link set dev ens33 name enp0s8
                sudo ip link set dev enp0s8 up
                sudo dhclient -pf /var/run/dhcp-enp0s8.pid enp0s8
              fi
              if sudo ip link | grep -q ens34
              then
                sudo ip link set dev ens34 down
                sudo ip link set dev ens34 name enp0s9
                sudo ip link set dev enp0s9 up
                sudo dhclient -pf /var/run/dhcp-enp0s9.pid enp0s9
              fi
            EOF
            s.args = []
          end

          # Run the provisioner after the last machine comes up
          override.vm.provision 'ansible', &ansible_provision if i == (NMONS - 1)
        end
      end

      mon.vm.provider :virtualbox do |vb, override|
        vb.linked_clone = true if Vagrant::VERSION =~ /^1.8/

        override.vm.network :private_network, ip: "#{SUBNET}.1#{i}", virtualbox__intnet: true
        override.vm.network :private_network, ip: "#{SUBNET}.2#{i}", virtualbox__intnet: true

        vb.customize ['createhd',
                      '--filename', "docker-#{i}",
                      '--size', '11000']
        # Controller names are dependent on the VM being built.
        # Be careful while changing the box.
        vb.customize ['storageattach', :id,
                      '--storagectl', 'SATA Controller',
                      '--port', 3,
                      '--type', 'hdd',
                      '--medium', "docker-#{i}.vdi"]

        (0..1).each do |d|
          disk_path = "disk-#{i}-#{d}"
          vdi_disk_path = disk_path + ".vdi"

          unless File.exist?(vdi_disk_path)
            vb.customize ['createhd',
                          '--filename', disk_path,
                          '--size', '11000']
            # Controller names are dependent on the VM being built.
            # Be careful while changing the box.
            vb.customize ['storageattach', :id,
                          '--storagectl', 'SATA Controller',
                          '--port', 4 + d,
                          '--type', 'hdd',
                          '--medium', vdi_disk_path]
          end
        end

        vb.customize ['modifyvm', :id, '--memory', "#{MEMORY}"]
        vb.customize ['modifyvm', :id, '--paravirtprovider', "kvm"]

        override.vm.provision "shell" do |s|
          s.inline = shell_provision
          s.args = [ ENV["http_proxy"] || "", ENV["https_proxy"] || "" ]
        end

        # Run the provisioner after the last machine comes up
        override.vm.provision 'ansible', &ansible_provision if i == (NMONS - 1)
      end
    end
  end
end
