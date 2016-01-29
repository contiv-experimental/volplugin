FROM ceph/rbd

ADD bin/volmaster /bin/volmaster
ADD bin/volplugin /bin/volplugin
ADD bin/volcli /bin/volcli
ADD bin/volsupervisor /bin/volsupervisor

ENTRYPOINT []
