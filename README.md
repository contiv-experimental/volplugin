# Ansible Playbooks

These are ansible playbooks we use for configuration management in a Contiv service cluster.

This project is used by vendoring into other repositories.

Following projects are using this work:

- **[contiv/build](https://github.com/contiv/build)** : uses it to generate vagrant boxes using packer
- **[contiv/lab](https://github.com/contiv/lab)** : uses it to configure dev and test host environments
- **[contiv/volplugin](https://github.com/contiv/volplugin)**: uses it to provision test vm environment
- **[contiv/cluster](https://github.com/contiv/cluster)** : uses it to manage the node commission/decommission workflow in a contiv cluster
