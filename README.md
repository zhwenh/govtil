govtil
======

Some util libraries in Go

muxconn
=======

muxconn splits a net.Conn into any number of proxy objects, each satisfying
the net.Conn interface and communicating over the same original connection.

Usage:

	import (
		"net"
		"github.com/vsekhar/govtil"
	)
	
	conn := net.Dial("tcp", "localhost:11235")
	muxconns := govtil.MuxConn(conn, 2)
	
	// Write independently to each split connection
	n, err := muxconns[0].Write([]byte{1, 1, 2, 3})
	n, err = muxconns[1].Write([]byte{5, 8, 13, 21})
	
	// Read independently from each split connection
	rdata1 := make([]byte, 5) 
	muxconns[0].Read(rdata1)
	rdata2 := make([]byte, 5)
	muxconns[1].Read(rdata2)
