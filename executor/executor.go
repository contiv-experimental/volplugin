// Package executor implements a high level execution context with monitoring and
// logging features.
//
// Please see Executor for more information.
package executor

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
)

// ExecResult is the result of a Wait() operation and contains various fields
// related to the post-mortem state of the process such as output and exit
// status.
type ExecResult struct {
	Stdout     string
	Stderr     string
	ExitStatus uint32
	Runtime    time.Duration

	executor *Executor
}

// Executor is the context used to execute a process. The runtime state is kept
// here. Please see the struct fields for more information.
//
// New() is the appropriate way to initialize this type.
//
// No attempt is made to manage concurrent requests to this struct after
// the program has started.
type Executor struct {
	// The interval at which we will log that we are still running.
	LogInterval time.Duration

	// If a true value is passed into this channel, the process will be forcefully
	// terminated. A false value terminates only the supervising goroutines, and
	// is used by Wait().
	TerminateChan chan bool

	// The function used for logging. Expects a format-style string and trailing args.
	LogFunc func(string, ...interface{})

	// The stdin as passed to the process.
	Stdin io.Reader

	timeout time.Duration

	command         *exec.Cmd
	stdout          io.ReadCloser
	stderr          io.ReadCloser
	startTime       time.Time
	terminateLogger chan struct{}
}

// New creates a new executor from an *exec.Cmd. You may modify the values
// before calling Start(). See Executor for more information.
func New(cmd *exec.Cmd) *Executor {
	return &Executor{
		LogInterval:   1 * time.Minute,
		TerminateChan: make(chan bool, 1),
		LogFunc:       logrus.Debugf,
		command:       cmd,
		stdout:        nil,
		stderr:        nil,
	}
}

func (e *Executor) String() string {
	return fmt.Sprintf("%v (%v) (pid: %v)", e.command.Args, e.command.Path, e.PID())
}

// SetTimeout sets the process timeout. If a non-zero timeout is provided, it
// will forcefully terminate the process after it is reached. It must be called
// before Start().
func (e *Executor) SetTimeout(t time.Duration) {
	e.timeout = t
}

// Start starts the command in the Executor context. It returns any error upon
// starting the process, but does not wait for it to complete. You may control
// it in a variety of ways (see Executor for more information).
func (e *Executor) Start() error {
	e.command.Stdin = e.Stdin

	e.startTime = time.Now()
	e.terminateLogger = make(chan struct{}, 1)

	var err error

	e.stdout, err = e.command.StdoutPipe()
	if err != nil {
		return err
	}

	e.stderr, err = e.command.StderrPipe()
	if err != nil {
		return err
	}

	if err := e.command.Start(); err != nil {
		e.LogFunc("Error executing %v: %v", e, err)
		return err
	}

	go e.waitForStop()

	if e.timeout != 0 {
		go e.terminateTimeout()
	}

	go e.logInterval()

	return nil
}

// TimeRunning returns the amount of time the program is or was running. Also
// see ExecResult.Runtime.
func (e *Executor) TimeRunning() time.Duration {
	return time.Now().Sub(e.startTime)
}

func (e *Executor) terminateTimeout() {
	time.Sleep(e.timeout)
	e.TerminateChan <- true
}

func (e *Executor) waitForStop() {
	terminate := <-e.TerminateChan

	if terminate {
		if e.command.Process == nil {
			e.LogFunc("Could not terminate non-running command %v", e)
		} else {
			e.LogFunc("Command %v terminated due to timeout. It may not have finished!", e)
			e.command.Process.Kill()
		}
	}

	e.terminateLogger <- struct{}{}
	return
}

func (e *Executor) logInterval() {
	for {
		time.Sleep(e.LogInterval)
		select {
		case <-e.terminateLogger:
			return
		default:
			e.LogFunc("%v has been running for %v", e, e.TimeRunning())
		}
	}
}

// PID yields the pid of the process (dead or alive), or 0 if the process has
// not been run yet.
func (e *Executor) PID() uint32 {
	if e.command.Process != nil {
		return uint32(e.command.Process.Pid)
	}

	return 0
}

// Wait waits for the process and return an ExecResult. The program died due to
// another problem without returning an exit status, a nil result is yielded.B
//
// If the ExecResult is returned, error will be nil, regardless of the
// ExitError returned by (*exec.Cmd).Wait(). If an error is returned it should
// be handled appropriately.
func (e *Executor) Wait() (*ExecResult, error) {
	stdout, err := ioutil.ReadAll(e.stdout)
	if err != nil {
		return nil, err
	}

	stderr, err := ioutil.ReadAll(e.stderr)
	if err != nil {
		return nil, err
	}

	res := &ExecResult{
		executor: e,
		Stdout:   string(stdout),
		Stderr:   string(stderr),
	}

	err = e.command.Wait()

	e.TerminateChan <- false // signal goroutines to terminate gracefully

	if err != nil {
		// if not exiterror, do not yield an execresult
		if exit, ok := err.(*exec.ExitError); ok {
			res.ExitStatus = uint32(exit.Sys().(syscall.WaitStatus))
		} else {
			return nil, err
		}
	}

	res.Runtime = e.TimeRunning()
	return res, nil
}

// Run calls Start(), then Wait(), and returns an ExecResult and error (if
// any). If an error is returned, ExecResult will be nil.
func (e *Executor) Run() (*ExecResult, error) {
	if err := e.Start(); err != nil {
		return nil, err
	}

	er, err := e.Wait()
	if err != nil {
		return nil, err
	}

	return er, nil
}

// Out returns an *os.File which is the stream of the standard output stream.
func (e *Executor) Out() io.ReadCloser {
	return e.stdout
}

// Err returns an io.ReadCloser which is the stream of the standard error stream.
func (e *Executor) Err() io.ReadCloser {
	return e.stderr
}

func (er *ExecResult) String() string {
	return fmt.Sprintf("Command: %v, Exit status %v, Runtime %v", er.executor.command.Args, er.ExitStatus, er.Runtime)
}
