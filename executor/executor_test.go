package executor

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	. "testing"
	"time"

	. "gopkg.in/check.v1"
)

type execSuite struct{}

var cmd = []string{"/bin/sleep", "200000000"}

var _ = Suite(&execSuite{})

func TestExec(t *T) {
	TestingT(t)
}

func (es *execSuite) TestProperties(c *C) {
	mycmd := exec.Command(cmd[0], cmd[1:]...)
	e := New(mycmd)
	c.Assert(e, NotNil)
	c.Assert(mycmd, DeepEquals, e.command)
	c.Assert(e.LogInterval, Equals, 1*time.Minute)
	c.Assert(e.TerminateChan, NotNil)
	c.Assert(e.Stdin, IsNil)
	c.Assert(e.timeout, Equals, time.Duration(0))
}

func (es *execSuite) TestStartWait(c *C) {
	e := New(exec.Command(cmd[0], cmd[1:]...))
	c.Assert(e, NotNil)
	c.Assert(e.Start(), IsNil)
	c.Assert(e.PID(), Not(Equals), uint32(0))
	c.Assert(e.command.Process, NotNil)
	// the sleep requires we signal the process since it'll sleepp forever.
	c.Assert(e.command.Process.Signal(syscall.SIGTERM), IsNil)
	er, err := e.Wait()
	c.Assert(err, IsNil)
	c.Assert(er, NotNil)
	c.Assert(er.ExitStatus, Equals, uint32(15))
	c.Assert(er.Runtime, Not(Equals), time.Duration(0))
}

func (es *execSuite) TestStdio(c *C) {
	cmd := exec.Command("/bin/echo", "yes")
	e := New(cmd)
	c.Assert(e.Start(), IsNil)
	time.Sleep(100 * time.Millisecond) // let it run a bit
	// yes runs eternally, so kill it manually.
	c.Assert(e.command.Process.Signal(syscall.SIGTERM), IsNil)
	er, _ := e.Wait()
	c.Assert(er, NotNil)
	c.Assert(er.Stdout, Matches, "yes\n")
	cmd = exec.Command("sh", "-c", "echo yes 1>&2")
	e = New(cmd)
	c.Assert(e.Start(), IsNil)
	time.Sleep(100 * time.Millisecond) // let it run a bit
	// yes runs eternally, so kill it manually.
	c.Assert(e.command.Process.Signal(syscall.SIGTERM), IsNil)
	er, _ = e.Wait()
	c.Assert(er, NotNil)
	c.Assert(er.Stderr, Matches, "yes\n")

	cmd = exec.Command("/bin/echo", "yes")
	e = New(cmd)
	c.Assert(e.Start(), IsNil)

	buf := new(bytes.Buffer)
	io.Copy(buf, e.Out())
	c.Assert(buf.String(), Equals, "yes\n")

	cmd = exec.Command("sh", "-c", "echo yes 1>&2")
	e = New(cmd)
	c.Assert(e.Start(), IsNil)

	buf = new(bytes.Buffer)
	io.Copy(buf, e.Err())
	c.Assert(buf.String(), Equals, "yes\n")

	cmd = exec.Command("cat")
	e = New(cmd)
	e.Stdin = bytes.NewBufferString("foo")
	c.Assert(e.Start(), IsNil)
	er, err := e.Wait()
	c.Assert(err, IsNil)
	c.Assert(er.Stdout, Equals, "foo")
}

func (es *execSuite) TestTimeout(c *C) {
	e := New(exec.Command(cmd[0], cmd[1:]...))
	results := []string{}
	loggerFunc := func(s string, args ...interface{}) {
		results = append(results, s)
	}

	e.LogFunc = loggerFunc
	e.SetTimeout(1 * time.Second)
	c.Assert(e.Start(), IsNil)
	time.Sleep(2 * time.Second)
	c.Assert(results[0], Equals, "Command %v terminated due to timeout. It may not have finished!")
	er, err := e.Wait()
	c.Assert(err, IsNil)
	c.Assert(er.ExitStatus, Not(Equals), 0)

	e = New(exec.Command("/bin/sleep", "2"))
	results = []string{}
	e.LogFunc = loggerFunc
	e.SetTimeout(4 * time.Second)
	er, err = e.Run()
	c.Assert(err, IsNil)
	c.Assert(er.ExitStatus, Equals, uint32(0))
	time.Sleep(2 * time.Second)
	c.Assert(len(results), Equals, 0)
}

func (es *execSuite) TestLogger(c *C) {
	results := []string{}
	loggerFunc := func(s string, args ...interface{}) {
		results = append(results, s)
	}

	cmd := exec.Command("/bin/sleep", "2")

	e := New(cmd)
	e.LogFunc = loggerFunc
	e.LogInterval = 1 * time.Second
	c.Assert(e.Start(), IsNil)
	time.Sleep(2 * time.Second)
	er, _ := e.Wait()
	c.Assert(er.ExitStatus, Equals, uint32(0))
	c.Assert(er.Runtime > 2*time.Second, Equals, true)
	// sometimes (depending on how long it takes to `cmd.Start()` launch) the
	// logger will log twice and other times it will only log once. To keep the
	// tests reliable, we only test the first log.
	c.Assert(results[0], Equals, "%v has been running for %v")
}

func (es *execSuite) TestString(c *C) {
	e := New(exec.Command(cmd[0], cmd[1:]...))
	c.Assert(e.String(), Equals, "[/bin/sleep 200000000] (/bin/sleep) (pid: 0)")
	c.Assert(e.Start(), IsNil)
	c.Assert(e.PID(), Not(Equals), uint32(0))
	c.Assert(e.String(), Equals, fmt.Sprintf("[/bin/sleep 200000000] (/bin/sleep) (pid: %v)", e.PID()))
	c.Assert(e.command.Process.Signal(syscall.SIGTERM), IsNil)
	er, _ := e.Wait()
	c.Assert(er, NotNil)
	c.Assert(er.ExitStatus, Equals, uint32(15))
}
