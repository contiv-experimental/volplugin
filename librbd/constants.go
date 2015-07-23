package librbd

import "path"

var (
	rbdBusPath    = "/sys/bus/rbd"
	rbdDevicePath = path.Join(rbdBusPath, "devices")
	rbdDev        = "/dev/rbd"
)
