package net

import (
	"io"
	"strings"
)

// Return whether the given error indicates a socket that produced it has been
// closed by the other end
func SocketClosed(err error) bool {
	if err == nil {
		return false
	}
	// TODO: update this with additional (perhaps non-TCP) checks
	// TODO: replace this with a check for net.errClosing when/if it's public
	errString := err.Error()
	if err == io.EOF ||
		strings.HasSuffix(errString, "use of closed network connection") ||
		strings.HasSuffix(errString, "broken pipe") ||
		strings.HasSuffix(errString, "connection reset by peer") {
		return true
	}
	return false
}
