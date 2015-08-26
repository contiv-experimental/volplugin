## librbd: interface to Ceph RADOS block devices

This provides facilities to Create, Delete, Map, and Unmap RBD Images. It
communicates with the librados/librbd C bindings and the krbd kernel module
directly. Most programs that leverage this library will wish to run as `root`.

## Example Usage

This code will:

1. Open a connection to the `rbd` pool
1. Creates an image called `test` (Removing it before if necessary)
1. Maps it to a block device on the local host
1. Creates a snapshot of the image, called `test-snap`.
1. Lists and prints the snapshots available.

```go
package main

import (
	"fmt"

	"github.com/contiv/librbd"
)

func main() {
	config := librbd.RBDConfig{
		MonitorIP: "86.75.30.9",
		UserName:  "admin",
		Secret:    "s3kr1t",
	}

	pool, err := GetPool(config, "rbd")
	if err != nil {
		panic(err)
	}

	if pool.ioctx == nil {
		panic("ioctx was nil")
	}

	if pool.cluster == nil {
		panic("rados was nil")
	}

	// it's ok if it fails, we just don't want the next call to unless stuff is
	// broken.
	pool.RemoveImage("test")
	if err := pool.CreateImage("test", 10000000); err != nil {
		panic(err)
	}

	img, err := pool.GetImage("test")
	if err != nil {
		panic(err)
	}

	device, err := img.MapDevice()
	if err != nil {
		panic(err)
	}

	if err := img.CreateSnapshot("test-snap"); err != nil {
		panic(err)
	}

	list, err := img.ListSnapshots(100)
	if err != nil {
		panic(err)
	}

	fmt.Println(list)
}
```

### Authors

Project Contiv: github.com/contiv

### License

Apache 2.0.
