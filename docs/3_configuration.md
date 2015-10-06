# Configuration

This section describes various ways to manipulate volplugin through
configuration and options.

## JSON Tenant Configuration

Tenant configuration uses JSON to configure the default volume parameters such
as what pool to use. It is uploaded to etcd by the `volcli` tool.

Here is an example:

```javascript
{
  "default-pool": "rbd",
  "default-options": {
    "size": 10,
    "snapshots": true,
    "snapshot": {
      "frequency": "30m",
      "keep": 20
    }
  }
}
```

Let's go through what these parameters mean.

* `default-pool`: the default ceph pool to install the images.
* `default-options`: the options that will be persisted unless overridden (see
	"Driver Options" below)
  * `size`: the size of the volume, in MB.
  * `snapshots`: use the snapshots facility.
  * `snapshot`: sub-level configuration for snapshots
		* `frequency`: the frequency between snapshots in Go's
			 [duration notation](https://golang.org/pkg/time/#ParseDuration)
		* `keep`: how many snapshots to keep

You supply them with `volcli tenant upload <tenant name>`. The JSON itself is
provided via standard input, so for example if your file is `tenant2.json`:

```
$ volcli tenant upload myTenant < tenant2.json
```

## Driver Options

Driver options are passed at `docker volume create` time with the `--opt` flag.
They are `key=value` pairs and are specified as such, f.e.:

```
docker volume create -d volplugin \
  --name tenant2/image \
  --opt size=1000
```

The options are as follows:

* `pool`: the pool to use for this volume.
* `size`: the size (in MB) for the volume.
* `snapshots`: take snapshots or not. Affects future options with `snapshot` in the key name.
  * the value must satisfy [this specification](https://golang.org/pkg/strconv/#ParseBool)
* `snapshots.frequency`: as above in the previous chapter, the frequency which we
  take snapshots.
* `snapshots.keep`: as above in the previous chapter, the number of snapshots to keep.
