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
    (0..2).each do |n|
        node_name = "host#{n}"
        config.vm.define node_name do |node|
            case n
            when 0
                node.vm.box = "puppetlabs/centos-7.2-64-nocm"
                node.vm.box_version = "1.0.1"
            when 1
                node.vm.box = "boxcutter/ubuntu1604"
                node.vm.box_version = "2.0.18"
            when 2
                node.vm.box = "boxcutter/ubuntu1510"
                node.vm.box_version = "2.0.18"
            end

            node.vm.provider "virtualbox" do |vb|
                vb.customize ['modifyvm', :id, '--memory', "4096"]
                vb.customize ["modifyvm", :id, "--cpus", "2"]
                vb.customize ['modifyvm', :id, '--paravirtprovider', 'kvm']
                vb.customize ['modifyvm', :id, '--natdnshostresolver1', 'on']
                vb.customize ['modifyvm', :id, '--natdnsproxy1', 'on']
                vb.linked_clone = true if Vagrant::VERSION =~ /^1.8/
            end

            if ansible_groups["devtest"] == nil then
                ansible_groups["devtest"] = [ ]
            end
            ansible_groups["devtest"] << node_name
            # Run the provisioner after all machines are up
            if n == 2 then
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
