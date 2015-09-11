# -*- mode: ruby -*-
# vi: set ft=ruby :

require 'yaml'
require 'fileutils'
VAGRANTFILE_API_VERSION = '2'

config_file=File.expand_path(File.join(File.dirname(__FILE__), 'vagrant_variables.yml'))
settings=YAML.load_file(config_file)

NMONS        = settings['mon_vms']
NOSDS        = settings['osd_vms']
NMDSS        = settings['mds_vms']
NRGWS        = settings['rgw_vms']
CLIENTS      = settings['client_vms']
SUBNET       = settings['subnet']
BOX          = settings['vagrant_box']
BOX_VERSION  = settings['box_version']
MEMORY       = settings['memory']

ansible_provision = proc do |ansible|
  ansible.playbook = 'ansible/site.yml'
  # Note: Can't do ranges like mon[0-2] in groups because
  # these aren't supported by Vagrant, see
  # https://github.com/mitchellh/vagrant/issues/3539
  ansible.groups = {
    'mons'        => (0..NMONS - 1).map { |j| "mon#{j}" },
    'restapis'    => (0..NMONS - 1).map { |j| "mon#{j}" },
    'osds'        => (0..NOSDS - 1).map { |j| "osd#{j}" },
    'mdss'        => (0..NMDSS - 1).map { |j| "mds#{j}" },
    'rgws'        => (0..NRGWS - 1).map { |j| "rgw#{j}" },
    'clients'     => (0..CLIENTS - 1).map { |j| "client#{j}" }
  }

  proxy_env = { }

  %w[http_proxy https_proxy].each do |name|
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

def create_vmdk(name, size)
  dir = Pathname.new(__FILE__).expand_path.dirname
  path = File.join(dir, '.vagrant', name + '.vmdk')
  `vmware-vdiskmanager -c -s #{size} -t 0 -a scsi #{path} \
   2>&1 > /dev/null` unless File.exist?(path)
end

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
  config.vm.box = BOX
  config.vm.box_version = BOX_VERSION
  config.ssh.insert_key = false # workaround for https://github.com/mitchellh/vagrant/issues/5048
  config.vm.synced_folder ".", "/opt/golang/src/github.com/contiv/volplugin"
  config.vm.synced_folder "systemtests/testdata", "/testdata"

  (0..CLIENTS - 1).each do |i|
    config.vm.define "client#{i}" do |client|
      client.vm.hostname = "ceph-client#{i}"
      client.vm.network :private_network, ip: "#{SUBNET}.4#{i}"
      client.vm.provider :virtualbox do |vb|
        vb.customize ['modifyvm', :id, '--memory', "#{MEMORY}"]
      end
      client.vm.provider :vmware_fusion do |v|
        v.vmx['memsize'] = "#{MEMORY}"
      end
    end
  end

  (0..NRGWS - 1).each do |i|
    config.vm.define "rgw#{i}" do |rgw|
      rgw.vm.hostname = "ceph-rgw#{i}"
      rgw.vm.network :private_network, ip: "#{SUBNET}.4#{i}"
      rgw.vm.provider :virtualbox do |vb|
        vb.customize ['modifyvm', :id, '--memory', "#{MEMORY}"]
      end
      rgw.vm.provider :vmware_fusion do |v|
        v.vmx['memsize'] = "#{MEMORY}"
      end
    end
  end

  (0..NMDSS - 1).each do |i|
    config.vm.define "mds#{i}" do |rgw|
      rgw.vm.hostname = "ceph-mds#{i}"
      rgw.vm.network :private_network, ip: "#{SUBNET}.7#{i}"
      rgw.vm.provider :virtualbox do |vb|
        vb.customize ['modifyvm', :id, '--memory', "#{MEMORY}"]
      end
      rgw.vm.provider :vmware_fusion do |v|
        v.vmx['memsize'] = "#{MEMORY}"
      end
    end
  end

  (0..NMONS - 1).each do |i|
    config.vm.define "mon#{i}" do |mon|
      mon.vm.hostname = "ceph-mon#{i}"
      mon.vm.network :private_network, ip: "#{SUBNET}.1#{i}"
      mon.vm.provider :virtualbox do |vb|
        vb.customize ['modifyvm', :id, '--memory', "#{MEMORY}"]
      end

      mon.vm.provider :vmware_fusion do |v|
        v.vmx['memsize'] = "#{MEMORY}"
      end
    end
  end

  (0..NOSDS - 1).each do |i|
    config.vm.define "osd#{i}" do |osd|
      osd.vm.hostname = "ceph-osd#{i}"
      osd.vm.network :private_network, ip: "#{SUBNET}.10#{i}"
      osd.vm.network :private_network, ip: "#{SUBNET}.20#{i}"
      osd.vm.provider :virtualbox do |vb|
        (0..1).each do |d|
          disk_path = "disk-#{i}-#{d}"
          if File.exist?(disk_path)
            puts "removing"
            FileUtils.rm_f(disk_path)
            puts File.exist?(disk_path)
          end
          #
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
                        '--medium', disk_path + ".vdi"]
        end

        vb.customize ['modifyvm', :id, '--memory', "#{MEMORY}"]
      end
      osd.vm.provider :vmware_fusion do |v|
        (0..1).each do |d|
          v.vmx["scsi0:#{d + 1}.present"] = 'TRUE'
          v.vmx["scsi0:#{d + 1}.fileName"] =
            create_vmdk("disk-#{i}-#{d}", '11000MB')
        end
        v.vmx['memsize'] = "#{MEMORY}"
      end

      # Run the provisioner after the last machine comes up
      config.vm.provision 'ansible', &ansible_provision if i == (NOSDS - 1)
    end
  end
end
