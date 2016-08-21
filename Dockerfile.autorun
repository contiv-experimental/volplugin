FROM docker:1.11.2

COPY systemtests/testdata/ceph/policy1.json /policy.json
COPY systemtests/testdata/globals/global1.json /global.json
COPY build/scripts/autorun-bootstrap.sh /bootstrap.sh
COPY build/scripts/build-volplugin-containers.sh /build.sh
RUN chmod +x /bootstrap.sh
ENTRYPOINT [ "/bin/sh", "/bootstrap.sh"  ]
