// Package testing provides utilities for testing
package testing

import (
	"net"
	"log"
)

// Set up a connection to myself
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
