package jail

import (
	"os/exec"
)

type chrootjail struct {
	fakedKillCh chan bool
	fakedKey int
	path string
}

// Imprison but DON'T chroot (needed during NewChrootJail())
func (c *chrootjail) imprison(cmd *exec.Cmd) *exec.Cmd {
	new_cmd := new(exec.Cmd)
	*new_cmd = *cmd
	// 1) set FAKEROOTKEY to c.fakedKey
	// 2) add path to libfakeroot-sysv.so and libfakechroot.so to front of
	//    LD_LIBRARY_PATH
	// 3) add libfakeroot-sysv.so and libfakechroot.so to front of LD_PRELOAD
	return new_cmd
}

func (c *chrootjail) Imprison(cmd *exec.Cmd) (*exec.Cmd, error) {
	new_cmd := c.imprison(cmd)
	// add chroot <path> /bin/bash to the command
	return new_cmd, nil
}

func (c *chrootjail) CleanUp() error {
	c.fakedKillCh <- true // kill
	<-c.fakedKillCh       // confirm
	return nil
}

// DO NOT USE THIS JAIL IN PRODUCTION CODE
//
// NewChrootJail creates a chroot-based jail. It requires paths to a directory
// in which the jailed filesystem will reside, and to an environment file where
// file permissions will be stored.
//
// This jail makes no attempt to secure administrative syscalls, memory access,
// or network sockets. It doesn't even really offer terribly good filesystem
// isolation. Consider using another jail based on lxc, seccomp, ptrace,
// SELinux, kernel Capabilities, etc., or all of the above.
func NewChrootJail(path string, envfile string) (Interface, error) {
	// start faked, one for each jail
	// TODO(vsekhar): consolidate into a singleton
	killch, key, err := faked(envfile, envfile)
	if err != nil {
		return nil, err
	}
	return &chrootjail{fakedKillCh: killch, fakedKey: key, path: path}, nil
}
