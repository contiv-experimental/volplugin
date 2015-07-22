package librbd

// #include <errno.h>
// #include <string.h>
//
import "C"
import (
	"errors"
	"os/exec"
)

func strerror(i C.int) error {
	return errors.New(C.GoString(C.strerror(-i)))
}

func modprobeRBD() error {
	return exec.Command("modprobe", "rbd").Run()
}
