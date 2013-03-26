// Package jail provides functions for creating and using ephemeral filesystem
// and execution jails for running untrusted code.
package jail

import (
	"os/exec"
)

// Interface for jails that can imprison commands, returning 'safe' versions
// that run within the jail. The environment, standard streams, file
// descriptors, and other configuration are preserved.
//
// Bootstrap() loads a standard filesystem
//
// Close() tidies up its context and saves any needed state.
type Interface interface {
	Bootstrap() error
	Imprison(c *exec.Cmd) *exec.Cmd
	Close() error
}
