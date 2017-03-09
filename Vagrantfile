# -*- mode: ruby -*-
# vi: set ft=ruby :

require 'yaml'
require 'fileutils'
VAGRANTFILE_API_VERSION = '2'
OSX_VMWARE_DIR = "/Applications/VMware Fusion.app/Contents/Library/"

config_file = File.expand_path(File.join(File.dirname(__FILE__), 'vagrant_variables.yml'))
ansible_config_file = File.expand_path(File.join(File.dirname(__FILE__), 'ansible', 'ansible.cfg'))
settings = YAML.load_file(config_file)

ENV['ANSIBLE_CONFIG'] = ansible_config_file

NMONS        = ENV["VMS"] || settings['vms']
BOX          = settings['vagrant_box']
BOX_VERSION  = settings['box_version']
MEMORY       = settings['memory']

SUBNET_PREFIX          = settings['subnet_prefix']
SUBNET_ASSIGNMENT_DIR  = "/tmp/volplugin_vagrant_subnets/"
SUBNET_ASSIGNMENT_FILE = File.expand_path(File.join(File.dirname(__FILE__), "subnet_assignment.state"))

class SubnetAssignmentFileError < Exception; end
class NoAvailableSubnetsError < Exception; end

# This method allocates a random subnet for Vagrant to use by combining the prefix
# specified in vagrant_variables.yml with a random third octet guaranteed to be
# unused by other environments.  If a previous subnet was allocated, it will be used.
#
# The subnet reservation process is locked, so multiple environments can be brought
# up in parallel with no issues.
def random_subnet
  begin
    Dir.mkdir(SUBNET_ASSIGNMENT_DIR)
  rescue Errno::EEXIST
  end

  if File.exists?(SUBNET_ASSIGNMENT_FILE)
    assigned_octet = File.read(SUBNET_ASSIGNMENT_FILE).strip

    # only use the existing assignment if the master assignment file still exists
    # i.e., the machine hasn't been rebooted
    if File.exists?(SUBNET_ASSIGNMENT_DIR + assigned_octet)
      return SUBNET_PREFIX + assigned_octet
    else
      msg = [
        "Subnet assignment database is missing.",
        "Are you trying to re-use an existing environment after a reboot?",
        "To continue, you should delete #{SUBNET_ASSIGNMENT_FILE}",
        "and rebuild the environment from scratch."
      ]
      raise SubnetAssignmentFileError.new(msg.join(" "))
    end
  end

  # no existing subnet assignment, so reserve a new one

  all_octets = (0..255).to_a.map(&:to_s)
  assigned_octet = nil

  loop do
    # filter out . and ..
    used_octets = Dir.entries(SUBNET_ASSIGNMENT_DIR).reject { |e| "." == e || ".." == e }

    # sanity check to make sure we can't loop forever
    if used_octets.size >= all_octets.size
      msg = [
        "All available subnets have been assigned.",
        "Delete #{SUBNET_ASSIGNMENT_DIR} or reboot to clear Vagrant's memory of what has been previously assigned."
      ]
      raise NoAvailableSubnetsError.new(msg.join(" "))
    end

    assigned_octet = (all_octets - used_octets).sample

    # make sure nothing else has taken the assigned_octet since our last directory check
    begin
      Dir.mkdir(SUBNET_ASSIGNMENT_DIR + assigned_octet)
    rescue Errno::EEXIST
      next
    end

    # subsequent invocations of vagrant will just use the octet from this file
    File.write(SUBNET_ASSIGNMENT_FILE, assigned_octet)
    break
  end

  SUBNET_PREFIX + assigned_octet
end

SUBNET   = random_subnet
NO_PROXY = [50,10,11,12].map { |n| "#{SUBNET}.#{n}" }.join(",")

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
    etcd_version: "v3.0.6",
    scheduler_provider: ENV["UCP"] ? "ucp-swarm" : "native-swarm",
    ucp_bootstrap_node_name: "mon0",
    ucp_license_remote: ENV["HOME"] + "/docker_subscription.lic",
    use_nfs_server: true,
    docker_version: "1.11.2",
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
    devices: "[ '/dev/sdd' ]",
    service_vip: "#{SUBNET}.50",
    consul_leader: "#{SUBNET}.10",
    journal_collocation: 'true',
    validate_certs: 'no',
    install_gluster: 'true',
    gluster_device: '/dev/sdc',
    gluster_interface: "enp0s8",
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

  config.ssh.insert_key = false

  config.vm.synced_folder ".", "/opt/golang/src/github.com/contiv/volplugin"
  config.vm.synced_folder "systemtests/testdata", "/testdata"
  config.vm.synced_folder "bin", "/tmp/bin"

  (0..NMONS-1).each do |i|
    config.vm.define "mon#{i}" do |mon|
      mon.vm.hostname = "mon#{i}"

      if ENV["UCP"] and mon.vm.hostname == "mon0"
        mon.vm.network "forwarded_port", guest: 443, host: 4443
      end

      if ENV["WEB"] and mon.vm.hostname == "mon0"
        mon.vm.network "forwarded_port", guest: 80, host: 8080
      end

      [:vmware_desktop, :vmware_workstation, :vmware_fusion].each do |provider|
        mon.vm.provider provider do |v, override|
          override.vm.network :private_network, type: "dhcp", ip: "#{SUBNET}.1#{i}", auto_config: false

          v.vmx["scsi0:1.present"] = 'TRUE'
          v.vmx["scsi0:1.fileName"] = create_vmdk("docker-#{i}", '15000MB')

          (1..2).each do |d|
            v.vmx["scsi0:#{d + 1}.present"] = 'TRUE'
            v.vmx["scsi0:#{d + 1}.fileName"] = create_vmdk("disk-#{i}-#{d}", '15000MB')
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

        disk_path = "docker-#{i}"
        vdi_disk_path = disk_path + ".vdi"

        unless File.exist?(vdi_disk_path)
          vb.customize ['createhd',
                        '--filename', disk_path,
                        '--size', '15000']

          # Controller names are dependent on the VM being built.
          # Be careful while changing the box.
          vb.customize ['storageattach', :id,
                        '--storagectl', 'SATA Controller',
                        '--port', 3,
                        '--type', 'hdd',
                        '--medium', vdi_disk_path]
        end

        (0..1).each do |d|
          disk_path = "disk-#{i}-#{d}"
          vdi_disk_path = disk_path + ".vdi"

          unless File.exist?(vdi_disk_path)
            vb.customize ['createhd',
                          '--filename', disk_path,
                          '--size', '15000']
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

        override.vm.provision 'shell' do |s|
          s.inline = <<-EOF
          ethtool -K enp0s3 gro off
          ethtool -K enp0s8 gro off
          EOF
          s.args = []
        end

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
