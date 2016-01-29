# -*- mode: ruby -*-
# vi: set ft=ruby :

# This Vagrantfile helps test the devtest role on base centos and ubuntu vms

host_env = { }
if ENV['CONTIV_ENV'] then
    ENV['CONTIV_ENV'].split(" ").each do |env|
        e = env.split("=")
        host_env[e[0]]=e[1]
    end
end

if ENV["http_proxy"]
  host_env["HTTP_PROXY"]  = host_env["http_proxy"]  = ENV["http_proxy"]
  host_env["HTTPS_PROXY"] = host_env["https_proxy"] = ENV["https_proxy"]
  host_env["NO_PROXY"]    = host_env["no_proxy"]    = ENV["no_proxy"]
end

ansible_groups = { }
ansible_playbook = ENV["CONTIV_ANSIBLE_PLAYBOOK"] || "./site.yml"
ansible_tags =  ENV["CONTIV_ANSIBLE_TAGS"] || "prebake-for-dev"
ansible_extra_vars = {
    "env" => host_env,
    "validate_certs" => "no",
}

puts "Host environment: #{host_env}"

Vagrant.configure(2) do |config|
    (0..1).each do |n|
        node_name = "host#{n}"
        config.vm.define node_name do |node|
            node.vm.hostname = node_name
            if n == 0 then
                node.vm.box = "puppetlabs/centos-7.2-64-nocm"
                node.vm.box_version = "1.0.0"
                node.vm.provision "shell" do |s|
                    #XXX: seems like the centos box has a broken packer binary which
                    #get's stuck on issuing 'packer --version' removing it manually here
                    s.inline = "rm -f /usr/sbin/packer"
                end
            else
                #node.vm.box = "puppetlabs/ubuntu-14.04-64-nocm"
                node.vm.box = "box-cutter/ubuntu1504"
                node.vm.box_version = "2.0.12"
            end

            # set the vm's ram and cpu big enough for docker to run fine
            node.vm.provider "virtualbox" do |vb|
                vb.customize ['modifyvm', :id, '--memory', "4096"]
                vb.customize ["modifyvm", :id, "--cpus", "2"]
            end

            if ansible_groups["devtest"] == nil then
                ansible_groups["devtest"] = [ ]
            end
            ansible_groups["devtest"] << node_name
            # Run the provisioner after all machines are up
            if n == 1 then
                node.vm.provision 'ansible' do |ansible|
                    ansible.groups = ansible_groups
                    ansible.playbook = ansible_playbook
                    ansible.extra_vars = ansible_extra_vars
                    ansible.limit = 'all'
                    ansible.tags = ansible_tags
                end
            end
        end
    end
end
