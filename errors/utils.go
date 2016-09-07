package errors

import (
	"github.com/contiv/errored"
	"github.com/coreos/etcd/client"
)

var etcdCodeTable = map[int]*errored.Error{
	client.ErrorCodeKeyNotFound:       NotExists,
	client.ErrorCodeNodeExist:         Exists,
	client.ErrorCodePrevValueRequired: LockMismatch,
	client.ErrorCodeTestFailed:        LockFailed,
}

// EtcdToErrored converts etcd errors to errored errors.
func EtcdToErrored(orig error) error {
	if ce, ok := orig.(client.Error); ok {
		err, ok := etcdCodeTable[ce.Code]
		if !ok {
			return ce
		}

		return err.Combine(ce)
	}

	return orig
}

// CombineError is a simplication of errored.Combine
func CombineError(err error, format string, args ...interface{}) error {
	if erd, ok := err.(*errored.Error); ok {
		erd.Combine(errored.Errorf(format, args...))
	}

	return errored.New(err.Error()).Combine(errored.Errorf(format, args...))
}
