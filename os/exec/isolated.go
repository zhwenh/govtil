// +build linux

package exec

/*
#cgo 
#include <signal.h>
#include <linux/sched.h>
#include <sys/syscall.h>

unsigned long cloneFlags = CLONE_NEWIPC
                         | CLONE_NEWUTS
                         | CLONE_NEWNS
                         | CLONE_NEWNET
                         | CLONE_NEWPID
                         | SIGCHLD;

int myClone() {
	return sys_clone(cloneFlags);
}
*/
import "C"

import (
	"bytes"
	"errors"
	"io"
	"os"
	osexec "os/exec"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"
)

// Here begins an unfortunate rehash of a portion of the standard library in
// order to modify parts to use kernel namespaces. Most functions are labelled
// with their original library source file, and the specific isolation changes
// are noted in forkAndExecInChild(...).

// VS: from syscall/exec_unix.go
// StartProcess wraps ForkExec for package os.
func sysStartProcess(argv0 string, argv []string, attr *syscall.ProcAttr) (pid int, handle uintptr, err error) {
	pid, err = forkExec(argv0, argv, attr)
	return pid, 0, err
}

// VS: from syscall/exec_unix.go
var zeroProcAttr syscall.ProcAttr
var zeroSysProcAttr syscall.SysProcAttr

// VS: from syscall/exec_unix.go
func forkExec(argv0 string, argv []string, attr *syscall.ProcAttr) (pid int, err error) {
	var p [2]int
	var n int
	var err1 syscall.Errno
	var wstatus syscall.WaitStatus

	if attr == nil {
		attr = &zeroProcAttr
	}
	sys := attr.Sys
	if sys == nil {
		sys = &zeroSysProcAttr
	}

	p[0] = -1
	p[1] = -1

	// Convert args to C form.
	argv0p, err := bytePtrFromString(argv0)
	if err != nil {
		return 0, err
	}
	argvp, err := slicePtrFromStrings(argv)
	if err != nil {
		return 0, err
	}
	envvp, err := slicePtrFromStrings(attr.Env)
	if err != nil {
		return 0, err
	}

	if runtime.GOOS == "freebsd" && len(argv[0]) > len(argv0) {
		argvp[0] = argv0p
	}

	var chroot *byte
	if sys.Chroot != "" {
		chroot, err = bytePtrFromString(sys.Chroot)
		if err != nil {
			return 0, err
		}
	}
	var dir *byte
	if attr.Dir != "" {
		dir, err = bytePtrFromString(attr.Dir)
		if err != nil {
			return 0, err
		}
	}

	// Acquire the fork lock so that no other threads
	// create new fds that are not yet close-on-exec
	// before we fork.
	syscall.ForkLock.Lock()

	// Allocate child status pipe close on exec.
	if err = syscall.Pipe(p[0:]); err != nil {
		goto error
	}
	if _, err = fcntl(p[0], syscall.F_SETFD, syscall.FD_CLOEXEC); err != nil {
		goto error
	}
	if _, err = fcntl(p[1], syscall.F_SETFD, syscall.FD_CLOEXEC); err != nil {
		goto error
	}

	// Kick off child.
	pid, err1 = forkAndExecInChild(argv0p, argvp, envvp, chroot, dir, attr, sys, p[1])
	if err1 != 0 {
		err = syscall.Errno(err1)
		goto error
	}
	syscall.ForkLock.Unlock()

	// Read child error status from pipe.
	syscall.Close(p[1])
	n, err = read(p[0], (*byte)(unsafe.Pointer(&err1)), int(unsafe.Sizeof(err1)))
	syscall.Close(p[0])
	if err != nil || n != 0 {
		if n == int(unsafe.Sizeof(err1)) {
			err = syscall.Errno(err1)
		}
		if err == nil {
			err = syscall.EPIPE
		}

		// Child failed; wait for it to exit, to make sure
		// the zombies don't accumulate.
		_, err1 := syscall.Wait4(pid, &wstatus, 0, nil)
		for err1 == syscall.EINTR {
			_, err1 = syscall.Wait4(pid, &wstatus, 0, nil)
		}
		return 0, err
	}

	// Read got EOF, so pipe closed on exec, so exec succeeded.
	return pid, nil

error:
	if p[0] >= 0 {
		syscall.Close(p[0])
		syscall.Close(p[1])
	}
	syscall.ForkLock.Unlock()
	return 0, err
}

// VS: from syscall/exec_linux.go
// Fork, dup fd onto 0..len(fd), and exec(argv0, argvv, envv) in child.
// If a dup or exec fails, write the errno error to pipe.
// (Pipe is close-on-exec so if exec succeeds, it will be closed.)
// In the child, this function must not acquire any locks, because
// they might have been locked at the time of the fork.  This means
// no rescheduling, no malloc calls, and no new stack segments.
// The calls to syscall.RawSyscall are okay because they are assembly
// functions that do not grow the stack.
func forkAndExecInChild(argv0 *byte, argv, envv []*byte, chroot, dir *byte, attr *syscall.ProcAttr, sys *syscall.SysProcAttr, pipe int) (pid int, err syscall.Errno) {
	// Declare all variables at top in case any
	// declarations require heap allocation (e.g., err1).
	var (
		r1     uintptr
		err1   syscall.Errno
		nextfd int
		i      int
	)

	// VS: >>>>> BEGIN ISOLATION STUFF
	// VS: <<<<< END ISOLATION STUFF

	// guard against side effects of shuffling fds below.
	fd := make([]int, len(attr.Files))
	for i, ufd := range attr.Files {
		fd[i] = int(ufd)
	}

	// About to call fork.
	// No more allocation or calls of non-assembly functions.
	// VS: >>>>> BEGIN ISOLATION STUFF
	const (
		CLONE_NEWNS   = 0x00020000
		CLONE_NEWUTS  = 0x04000000
		CLONE_NEWIPC  = 0x08000000
		CLONE_NEWUSER = 0x10000000
		CLONE_NEWPID  = 0x20000000
		CLONE_NEWNET  = 0x40000000
		SIGCHLD       = 0x11
	)
	var cloneFlag uintptr
	cloneFlag = CLONE_NEWNS | CLONE_NEWUTS | CLONE_NEWIPC | CLONE_NEWUSER | CLONE_NEWPID | CLONE_NEWNET | SIGCHLD
	//r1, _, err1 = syscall.RawSyscall(syscall.SYS_CLONE, cloneFlag, 0, 0)
	r1, err1 = C.myClone()
	// VS: <<<<< END ISOLATION STUFF
	// r1, _, err1 = syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
	if err1 != 0 {
		return 0, err1
	}

	if r1 != 0 {
		// parent; return PID
		return int(r1), 0
	}

	// Fork succeeded, now in child.

	// Parent death signal
	if sys.Pdeathsig != 0 {
		_, _, err1 = syscall.RawSyscall6(syscall.SYS_PRCTL, syscall.PR_SET_PDEATHSIG, uintptr(sys.Pdeathsig), 0, 0, 0, 0)
		if err1 != 0 {
			goto childerror
		}

		// Signal self if parent is already dead. This might cause a
		// duplicate signal in rare cases, but it won't matter when
		// using SIGKILL.
		r1, _, _ = syscall.RawSyscall(syscall.SYS_GETPPID, 0, 0, 0)
		if r1 == 1 {
			pid, _, _ := syscall.RawSyscall(syscall.SYS_GETPID, 0, 0, 0)
			_, _, err1 := syscall.RawSyscall(syscall.SYS_KILL, pid, uintptr(sys.Pdeathsig), 0)
			if err1 != 0 {
				goto childerror
			}
		}
	}

	// Enable tracing if requested.
	if sys.Ptrace {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_PTRACE, uintptr(syscall.PTRACE_TRACEME), 0, 0)
		if err1 != 0 {
			goto childerror
		}
	}

	// Session ID
	if sys.Setsid {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_SETSID, 0, 0, 0)
		if err1 != 0 {
			goto childerror
		}
	}

	// Set process group
	if sys.Setpgid {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_SETPGID, 0, 0, 0)
		if err1 != 0 {
			goto childerror
		}
	}

	// Chroot
	if chroot != nil {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_CHROOT, uintptr(unsafe.Pointer(chroot)), 0, 0)
		if err1 != 0 {
			goto childerror
		}
	}

	// User and groups
	if cred := sys.Credential; cred != nil {
		ngroups := uintptr(len(cred.Groups))
		groups := uintptr(0)
		if ngroups > 0 {
			groups = uintptr(unsafe.Pointer(&cred.Groups[0]))
		}
		_, _, err1 = syscall.RawSyscall(syscall.SYS_SETGROUPS, ngroups, groups, 0)
		if err1 != 0 {
			goto childerror
		}
		_, _, err1 = syscall.RawSyscall(syscall.SYS_SETGID, uintptr(cred.Gid), 0, 0)
		if err1 != 0 {
			goto childerror
		}
		_, _, err1 = syscall.RawSyscall(syscall.SYS_SETUID, uintptr(cred.Uid), 0, 0)
		if err1 != 0 {
			goto childerror
		}
	}

	// Chdir
	if dir != nil {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_CHDIR, uintptr(unsafe.Pointer(dir)), 0, 0)
		if err1 != 0 {
			goto childerror
		}
	}

	// Pass 1: look for fd[i] < i and move those up above len(fd)
	// so that pass 2 won't stomp on an fd it needs later.
	nextfd = int(len(fd))
	if pipe < nextfd {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_DUP2, uintptr(pipe), uintptr(nextfd), 0)
		if err1 != 0 {
			goto childerror
		}
		syscall.RawSyscall(syscall.SYS_FCNTL, uintptr(nextfd), syscall.F_SETFD, syscall.FD_CLOEXEC)
		pipe = nextfd
		nextfd++
	}
	for i = 0; i < len(fd); i++ {
		if fd[i] >= 0 && fd[i] < int(i) {
			_, _, err1 = syscall.RawSyscall(syscall.SYS_DUP2, uintptr(fd[i]), uintptr(nextfd), 0)
			if err1 != 0 {
				goto childerror
			}
			syscall.RawSyscall(syscall.SYS_FCNTL, uintptr(nextfd), syscall.F_SETFD, syscall.FD_CLOEXEC)
			fd[i] = nextfd
			nextfd++
			if nextfd == pipe { // don't stomp on pipe
				nextfd++
			}
		}
	}

	// Pass 2: dup fd[i] down onto i.
	for i = 0; i < len(fd); i++ {
		if fd[i] == -1 {
			syscall.RawSyscall(syscall.SYS_CLOSE, uintptr(i), 0, 0)
			continue
		}
		if fd[i] == int(i) {
			// dup2(i, i) won't clear close-on-exec flag on Linux,
			// probably not elsewhere either.
			_, _, err1 = syscall.RawSyscall(syscall.SYS_FCNTL, uintptr(fd[i]), syscall.F_SETFD, 0)
			if err1 != 0 {
				goto childerror
			}
			continue
		}
		// The new fd is created NOT close-on-exec,
		// which is exactly what we want.
		_, _, err1 = syscall.RawSyscall(syscall.SYS_DUP2, uintptr(fd[i]), uintptr(i), 0)
		if err1 != 0 {
			goto childerror
		}
	}

	// By convention, we don't close-on-exec the fds we are
	// started with, so if len(fd) < 3, close 0, 1, 2 as needed.
	// Programs that know they inherit fds >= 3 will need
	// to set them close-on-exec.
	for i = len(fd); i < 3; i++ {
		syscall.RawSyscall(syscall.SYS_CLOSE, uintptr(i), 0, 0)
	}

	// Detach fd 0 from tty
	if sys.Noctty {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_IOCTL, 0, uintptr(syscall.TIOCNOTTY), 0)
		if err1 != 0 {
			goto childerror
		}
	}

	// Make fd 0 the tty
	if sys.Setctty {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_IOCTL, 0, uintptr(syscall.TIOCSCTTY), 0)
		if err1 != 0 {
			goto childerror
		}
	}

	// Time to exec.
	_, _, err1 = syscall.RawSyscall(syscall.SYS_EXECVE,
		uintptr(unsafe.Pointer(argv0)),
		uintptr(unsafe.Pointer(&argv[0])),
		uintptr(unsafe.Pointer(&envv[0])))

childerror:
	// send error code on pipe
	syscall.RawSyscall(syscall.SYS_WRITE, uintptr(pipe), uintptr(unsafe.Pointer(&err1)), unsafe.Sizeof(err1))
	for {
		syscall.RawSyscall(syscall.SYS_EXIT, 253, 0, 0)
	}

	// Calling panic is not actually safe,
	// but the for loop above won't break
	// and this shuts up the compiler.
	panic("unreached")
}

// VS: from os/doc.go
// StartProcess starts a new process with the program, arguments and attributes
// specified by name, argv and attr.
//
// StartProcess is a low-level interface. The os/exec package provides
// higher-level interfaces.
//
// If there is an error, it will be of type *PathError.
func StartProcess(name string, argv []string, attr *os.ProcAttr) (*os.Process, error) {
	return startProcess(name, argv, attr)
}

// VS: from os/exec_posix.go
func startProcess(name string, argv []string, attr *os.ProcAttr) (p *os.Process, err error) {
	// If there is no SysProcAttr (ie. no Chroot or changed
	// UID/GID), double-check existence of the directory we want
	// to chdir into.  We can make the error clearer this way.
	if attr != nil && attr.Sys == nil && attr.Dir != "" {
		if _, err := os.Stat(attr.Dir); err != nil {
			pe := err.(*os.PathError)
			pe.Op = "chdir"
			return nil, pe
		}
	}

	sysattr := &syscall.ProcAttr{
		Dir: attr.Dir,
		Env: attr.Env,
		Sys: attr.Sys,
	}
	if sysattr.Env == nil {
		sysattr.Env = os.Environ()
	}
	for _, f := range attr.Files {
		sysattr.Files = append(sysattr.Files, f.Fd())
	}

	pid, h, e := sysStartProcess(name, argv, sysattr)
	if e != nil {
		return nil, &os.PathError{"fork/exec", name, e}
	}
	return newProcess(pid, h), nil
}

// VS: from os/exec.go
func newProcess(pid int, handle uintptr) *os.Process {
	p := &os.Process{Pid: pid /*, handle: handle*/}
	runtime.SetFinalizer(p, (*os.Process).Release)
	return p
}

// VS: from syscall/zsyscall_linux_amd64.go
func fcntl(fd int, cmd int, arg int) (val int, err error) {
	r0, _, e1 := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(cmd), uintptr(arg))
	val = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

// VS: from syscall/zsyscall_linux_amd64.go
func read(fd int, p *byte, np int) (n int, err error) {
	r0, _, e1 := syscall.Syscall(syscall.SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(p)), uintptr(np))
	n = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

// VS: from syscall/syscall.go
// byteSliceFromString returns a NUL-terminated slice of bytes
// containing the text of s. If s contains a NUL byte at any
// location, it returns (nil, EINVAL).
func byteSliceFromString(s string) ([]byte, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return nil, syscall.EINVAL
		}
	}
	a := make([]byte, len(s)+1)
	copy(a, s)
	return a, nil
}

// VS: from syscall/syscall.go
// bytePtrFromString returns a pointer to a NUL-terminated array of
// bytes containing the text of s. If s contains a NUL byte at any
// location, it returns (nil, EINVAL).
func bytePtrFromString(s string) (*byte, error) {
	a, err := byteSliceFromString(s)
	if err != nil {
		return nil, err
	}
	return &a[0], nil
}

// VS: from syscall/exec_unix.go
// slicePtrFromStrings converts a slice of strings to a slice of
// pointers to NUL-terminated byte slices. If any string contains
// a NUL byte, it returns (nil, EINVAL).
func slicePtrFromStrings(ss []string) ([]*byte, error) {
	var err error
	bb := make([]*byte, len(ss)+1)
	for i := 0; i < len(ss); i++ {
		bb[i], err = bytePtrFromString(ss[i])
		if err != nil {
			return nil, err
		}
	}
	bb[len(ss)] = nil
	return bb, nil
}

//////////////////////////
// from os/exec/exec.go //
//////////////////////////

// Error records the name of a binary that failed to be executed
// and the reason it failed.
type Error struct {
	Name string
	Err  error
}

func (e *Error) Error() string {
	return "exec: " + strconv.Quote(e.Name) + ": " + e.Err.Error()
}

// Cmd represents an external command being prepared or run.
type Cmd struct {
	// Path is the path of the command to run.
	//
	// This is the only field that must be set to a non-zero
	// value.
	Path string

	// Args holds command line arguments, including the command as Args[0].
	// If the Args field is empty or nil, Run uses {Path}.
	// 
	// In typical use, both Path and Args are set by calling Command.
	Args []string

	// Env specifies the environment of the process.
	// If Env is nil, Run uses the current process's environment.
	Env []string

	// Dir specifies the working directory of the command.
	// If Dir is the empty string, Run runs the command in the
	// calling process's current directory.
	Dir string

	// Stdin specifies the process's standard input. If Stdin is
	// nil, the process reads from the null device (os.DevNull).
	Stdin io.Reader

	// Stdout and Stderr specify the process's standard output and error.
	//
	// If either is nil, Run connects the corresponding file descriptor
	// to the null device (os.DevNull).
	//
	// If Stdout and Stderr are the same writer, at most one
	// goroutine at a time will call Write.
	Stdout io.Writer
	Stderr io.Writer

	// ExtraFiles specifies additional open files to be inherited by the
	// new process. It does not include standard input, standard output, or
	// standard error. If non-nil, entry i becomes file descriptor 3+i.
	//
	// BUG: on OS X 10.6, child processes may sometimes inherit unwanted fds.
	// http://golang.org/issue/2603
	ExtraFiles []*os.File

	// SysProcAttr holds optional, operating system-specific attributes.
	// Run passes it to os.StartProcess as the os.ProcAttr's Sys field.
	SysProcAttr *syscall.SysProcAttr

	// Process is the underlying process, once started.
	Process *os.Process

	// ProcessState contains information about an exited process,
	// available after a call to Wait or Run.
	ProcessState *os.ProcessState

	err             error // last error (from LookPath, stdin, stdout, stderr)
	finished        bool  // when Wait was called
	childFiles      []*os.File
	closeAfterStart []io.Closer
	closeAfterWait  []io.Closer
	goroutine       []func() error
	errch           chan error // one send per goroutine
}

// Command returns the Cmd struct to execute the named program with
// the given arguments.
//
// It sets Path and Args in the returned structure and zeroes the
// other fields.
//
// If name contains no path separators, Command uses LookPath to
// resolve the path to a complete name if possible. Otherwise it uses
// name directly.
//
// The returned Cmd's Args field is constructed from the command name
// followed by the elements of arg, so arg should not include the
// command name itself. For example, Command("echo", "hello")
func Command(name string, arg ...string) *Cmd {
	aname, err := osexec.LookPath(name)
	if err != nil {
		aname = name
	}
	return &Cmd{
		Path: aname,
		Args: append([]string{name}, arg...),
		err:  err,
	}
}

// interfaceEqual protects against panics from doing equality tests on
// two interfaces with non-comparable underlying types
func interfaceEqual(a, b interface{}) bool {
	defer func() {
		recover()
	}()
	return a == b
}

func (c *Cmd) envv() []string {
	if c.Env != nil {
		return c.Env
	}
	return os.Environ()
}

func (c *Cmd) argv() []string {
	if len(c.Args) > 0 {
		return c.Args
	}
	return []string{c.Path}
}

func (c *Cmd) stdin() (f *os.File, err error) {
	if c.Stdin == nil {
		f, err = os.Open(os.DevNull)
		if err != nil {
			return
		}
		c.closeAfterStart = append(c.closeAfterStart, f)
		return
	}

	if f, ok := c.Stdin.(*os.File); ok {
		return f, nil
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return
	}

	c.closeAfterStart = append(c.closeAfterStart, pr)
	c.closeAfterWait = append(c.closeAfterWait, pw)
	c.goroutine = append(c.goroutine, func() error {
		_, err := io.Copy(pw, c.Stdin)
		if err1 := pw.Close(); err == nil {
			err = err1
		}
		return err
	})
	return pr, nil
}

func (c *Cmd) stdout() (f *os.File, err error) {
	return c.writerDescriptor(c.Stdout)
}

func (c *Cmd) stderr() (f *os.File, err error) {
	if c.Stderr != nil && interfaceEqual(c.Stderr, c.Stdout) {
		return c.childFiles[1], nil
	}
	return c.writerDescriptor(c.Stderr)
}

func (c *Cmd) writerDescriptor(w io.Writer) (f *os.File, err error) {
	if w == nil {
		f, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			return
		}
		c.closeAfterStart = append(c.closeAfterStart, f)
		return
	}

	if f, ok := w.(*os.File); ok {
		return f, nil
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return
	}

	c.closeAfterStart = append(c.closeAfterStart, pw)
	c.closeAfterWait = append(c.closeAfterWait, pr)
	c.goroutine = append(c.goroutine, func() error {
		_, err := io.Copy(w, pr)
		return err
	})
	return pw, nil
}

func (c *Cmd) closeDescriptors(closers []io.Closer) {
	for _, fd := range closers {
		fd.Close()
	}
}

// Run starts the specified command and waits for it to complete.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the command fails to run or doesn't complete successfully, the
// error is of type *ExitError. Other error types may be
// returned for I/O problems.
func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Start starts the specified command but does not wait for it to complete.
func (c *Cmd) Start() error {
	if c.err != nil {
		return c.err
	}
	if c.Process != nil {
		return errors.New("exec: already started")
	}

	type F func(*Cmd) (*os.File, error)
	for _, setupFd := range []F{(*Cmd).stdin, (*Cmd).stdout, (*Cmd).stderr} {
		fd, err := setupFd(c)
		if err != nil {
			c.closeDescriptors(c.closeAfterStart)
			c.closeDescriptors(c.closeAfterWait)
			return err
		}
		c.childFiles = append(c.childFiles, fd)
	}
	c.childFiles = append(c.childFiles, c.ExtraFiles...)

	var err error
	c.Process, err = /*os.*/ StartProcess(c.Path, c.argv(), &os.ProcAttr{
		Dir:   c.Dir,
		Files: c.childFiles,
		Env:   c.envv(),
		Sys:   c.SysProcAttr,
	})
	if err != nil {
		c.closeDescriptors(c.closeAfterStart)
		c.closeDescriptors(c.closeAfterWait)
		return err
	}

	c.closeDescriptors(c.closeAfterStart)

	c.errch = make(chan error, len(c.goroutine))
	for _, fn := range c.goroutine {
		go func(fn func() error) {
			c.errch <- fn()
		}(fn)
	}

	return nil
}

// An ExitError reports an unsuccessful exit by a command.
type ExitError struct {
	*os.ProcessState
}

func (e *ExitError) Error() string {
	return e.ProcessState.String()
}

// Wait waits for the command to exit.
// It must have been started by Start.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the command fails to run or doesn't complete successfully, the
// error is of type *ExitError. Other error types may be
// returned for I/O problems.
func (c *Cmd) Wait() error {
	if c.Process == nil {
		return errors.New("exec: not started")
	}
	if c.finished {
		return errors.New("exec: Wait was already called")
	}
	c.finished = true
	state, err := c.Process.Wait()
	c.ProcessState = state

	var copyError error
	for _ = range c.goroutine {
		if err := <-c.errch; err != nil && copyError == nil {
			copyError = err
		}
	}

	c.closeDescriptors(c.closeAfterWait)

	if err != nil {
		return err
	} else if !state.Success() {
		return &ExitError{state}
	}

	return copyError
}

// Output runs the command and returns its standard output.
func (c *Cmd) Output() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	err := c.Run()
	return b.Bytes(), err
}

// CombinedOutput runs the command and returns its combined standard
// output and standard error.
func (c *Cmd) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	return b.Bytes(), err
}

// StdinPipe returns a pipe that will be connected to the command's
// standard input when the command starts.
func (c *Cmd) StdinPipe() (io.WriteCloser, error) {
	if c.Stdin != nil {
		return nil, errors.New("exec: Stdin already set")
	}
	if c.Process != nil {
		return nil, errors.New("exec: StdinPipe after process started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stdin = pr
	c.closeAfterStart = append(c.closeAfterStart, pr)
	c.closeAfterWait = append(c.closeAfterWait, pw)
	return pw, nil
}

// StdoutPipe returns a pipe that will be connected to the command's
// standard output when the command starts.
// The pipe will be closed automatically after Wait sees the command exit.
func (c *Cmd) StdoutPipe() (io.ReadCloser, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Process != nil {
		return nil, errors.New("exec: StdoutPipe after process started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stdout = pw
	c.closeAfterStart = append(c.closeAfterStart, pw)
	c.closeAfterWait = append(c.closeAfterWait, pr)
	return pr, nil
}

// StderrPipe returns a pipe that will be connected to the command's
// standard error when the command starts.
// The pipe will be closed automatically after Wait sees the command exit.
func (c *Cmd) StderrPipe() (io.ReadCloser, error) {
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	if c.Process != nil {
		return nil, errors.New("exec: StderrPipe after process started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stderr = pw
	c.closeAfterStart = append(c.closeAfterStart, pw)
	c.closeAfterWait = append(c.closeAfterWait, pr)
	return pr, nil
}
