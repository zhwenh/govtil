// Package jail provides functions for creating and using ephemeral filesystem
// and execution jails for running untrusted code.
package jail

import (
	"os/exec"
)

// Interface for jails that can imprison commands, returning 'safe' versions
// that run within the jail. The environment, standard streams, file
// descriptors, and other configuration inside exec.Cmd are respected.
//
// CleanUp() tidies up any system state generated by the jail. Using a jail
// after calling CleanUp() results in undefined behavior.
type Interface interface {
	Imprison(c *exec.Cmd) (*exec.Cmd, error)
	CleanUp() error
}

// Create an exec.Cmd object based on a BASH-parsable command string.
// Use this instead of os/exec.Command() as the standard version will mangle the
// command path based on the configuration of the host machine.
func Command(cmd string) *exec.Cmd {
	return &exec.Cmd{Path: cmd}
}
