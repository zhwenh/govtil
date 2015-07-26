// Package testing provides utilities for testing
package testing

import (
	"errors"
	"log"
	"net"
	"runtime"
)

// Set up a connection to myself via ephemeral ports
func SelfConnection() (net.Conn, net.Conn) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("Could not set up listen: ", err)
	}
	defer listener.Close()

	inconnch := make(chan net.Conn)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("Couldn't receive connection")
		}
		inconnch <- conn
	}()

	outconn, _ := net.Dial("tcp", listener.Addr().String())
	inconn := <-inconnch
	return inconn, outconn
}

type RPCRecv int

func (r *RPCRecv) Echo(in *string, out *string) error {
	*out = *in
	return nil
}

func (r *RPCRecv) Error(*string, *string) error {
	return errors.New("testing.RPCRecv intentional error")
}

var stackBuf = make([]byte, 4096)

// Get stack trace (don't use in panic situations as this function allocs)
func Stack() string {
	n := runtime.Stack(stackBuf, false)
	return string(stackBuf[:n])
}
