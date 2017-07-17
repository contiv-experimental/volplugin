FROM ceph/rbd

COPY ["bin/apiserver", "bin/volplugin",  "bin/volcli", "bin/volsupervisor", "/bin/"]

ENV PATH /opt/bin:$PATH

ENTRYPOINT []
