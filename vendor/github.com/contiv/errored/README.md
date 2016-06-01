[![ReportCard][ReportCard-Image]][ReportCard-URL] [![Build][Build-Status-Image]][Build-Status-URL] [![GoDoc][GoDoc-Image]][GoDoc-URL]

# Errored: flexible error messages for golang

Package errored implements specialized errors for golang that come with:

* Debug and Trace modes
  * Debug emits the location the error was created, Trace emits the whole stack.
* Error combination
  * Make two errors into one; carries the trace information for both errors with it!

Use it just like `fmt`:

```go
package main

import "github.com/contiv/errored"

func main() {
	err := errored.Errorf("a message")
	err.SetDebug(true)
	err.Error() // => "a message [ <file> <line> <line number> ]"
	err2 := errored.Errorf("another message")
	combined := err.Combine(err2)
	combined.SetTrace(true)
	combined.Error() // => "a message: another message" + two stack traces
  combined.Contains(err2) // true
}
```

## Authors:

* Madhav Puri
* Erik Hollensbe

## Sponsorship

Project Contiv is sponsored by Cisco Systems, Inc.

[ReportCard-URL]: https://goreportcard.com/report/github.com/contiv/errored
[ReportCard-Image]: http://goreportcard.com/badge/contiv/errored
[Build-Status-URL]: http://travis-ci.org/contiv/errored
[Build-Status-Image]: https://travis-ci.org/contiv/errored.svg?branch=master
[GoDoc-URL]: https://godoc.org/github.com/contiv/errored
[GoDoc-Image]: https://godoc.org/github.com/contiv/errored?status.svg
