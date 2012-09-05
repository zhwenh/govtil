// Package net provides tools and helpers for network servers and connectivity
package net

import (
	"io"
	"net"
	"os"
	"os/signal"
	"strings"

	"github.com/vsekhar/govtil/log"
)

// Return whether the given error indicates a socket that produced it has been
// closed by the other end
//
// Currently, SocketClosed() will return true if err:
//	== io.EOF
//	.Error() end in:
//		"use of closed network connection"
//		"broken pipe"
//		"connection reset by peer"
//
// Eventually, SocketClosed() will replace the string comparisons with a test
// for net.errClosing when/if it is made public
//
func SocketClosed(err error) bool {
	if err == nil {
		return false
	}
	// TODO: update this with additional (perhaps non-TCP) checks
	errString := err.Error()
	if err == io.EOF ||
		strings.HasSuffix(errString, "use of closed network connection") ||
		strings.HasSuffix(errString, "broken pipe") ||
		strings.HasSuffix(errString, "connection reset by peer") {
		return true
	}
	return false
}

// Creates a listener that is closed in the event any signal listed is received
// by the process. This is useful as a way to gracefully close an HTTP or RPC
// server.
func Listen(proto string, addr string, signals ...os.Signal) (l net.Listener, err error) {
	l, err = net.Listen(proto, addr)
	if err != nil {
		return
	}
	sigch := make(chan os.Signal)
	signal.Notify(sigch, signals...)
	go func() {
		sig := <-sigch
		log.Println("Closing listen port", l.Addr().String(), "due to signal", sig)
		l.Close()
	}()
	return
}
