// Package net provides tools and helpers for network servers and connectivity
package net

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/vsekhar/govtil/log"
	"github.com/vsekhar/govtil/os/signal"
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

// Signallistener returns a net.Listener that will be closed when standard OS
// stop signals are received.
func SignalListener(port int) (net.Listener, error) {
	addr := ":" + fmt.Sprint(port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Errorln("govtil/net: Failed to listen on", port, err)
		return nil, err
	}

	// Close listen port on signals (causes http.Serve() to return)
	signal.Go(func(s os.Signal) {
		log.Debugf("govtil/net: Closing listen port %v due to signal %v", l.Addr().String(), s)
		l.Close()
	})

	return l, nil
}
