// Package testing provides utilities for testing
package testing

import (
	"errors"
	//"log"
	"net"
	"strconv"

	"github.com/vsekhar/govtil/log"
	vnet "github.com/vsekhar/govtil/net"
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

func LocalListener() (net.Listener, int, error) {
	l, err := vnet.SignalListener(0)
	if err != nil {
		return nil, 0, err
	}
	_, aps, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return nil, 0, err
	}
	port, err := strconv.Atoi(aps)
	if err != nil {
		return nil, 0, err
	}
	return l, port, nil
}

type RPCRecv int

func (r *RPCRecv) Echo(in *string, out *string) error {
	*out = *in
	return nil
}

func (r *RPCRecv) Error(*string, *string) error {
	return errors.New("testing.RPCRecv intentional error")
}

