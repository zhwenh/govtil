// Package exec provides helpers for running subprocesses with inherited
// network socket file descriptors
package exec

import (
	"errors"
	"io"
	"os"
)

type fileable interface {
	File() (f *os.File, err error)
}

// FileFromConn extracts a file descriptor from an object such as a network
// socket. The argument must have a File() method that returns (*os.File, error).
func FileFromConn(c interface{}) (*os.File, error) {
	f, ok := c.(fileable)
	if !ok {
		return nil, errors.New("Cannot get os.File from connection; ensure it is a TCP/UDP/Unix socket")
	}
	return f.File()
}

// Interface matching that of the os/exec.Cmd struct, for returning Cmd-like
// objects.
type Cmd interface {
	CombinedOutput() ([]byte, error)
	Output() ([]byte, error)
	Run() error
	Start() error
	StderrPipe() (io.ReadCloser, error)
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	Wait() error
}
