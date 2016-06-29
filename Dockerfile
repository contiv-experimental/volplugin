FROM ceph/rbd

ADD bin/apiserver /bin/apiserver
ADD bin/volplugin /bin/volplugin
ADD bin/volcli /bin/volcli
ADD bin/volsupervisor /bin/volsupervisor

ENTRYPOINT []
