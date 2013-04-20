// Package exec provides helpers for running subprocesses with inherited
// network socket file descriptors (linux only)

// +build linux

// Running in an isolated namespace
//   1) Create and configure temporary cgroup
//   2) Fork (parent listens for error code, returns)
//   3) Attach to cgroup
//   4) cd, chroot, etc.
//   5) unshare(CLONE_ALLNS)
//   6) if error, emit code on pipe and exit
//   7) emit ok
//   8) exec

package exec

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

// Run is a convenience function to quickly run a command and check for errors.
// If running the command produces an error, the error is returned along with
// the combined output of the command.
func Run(cmd string, args ...string) error {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return fmt.Errorf("command not found: %s", cmd)
	}
	co, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s %v\n%s", cmd, cmd, strings.Join(args, " "), co)
	}
	return nil
}
