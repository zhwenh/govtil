// Package birpc provides a bi-directional RPC handler. Incoming connections
// are muxed, with one connection going to the RPC server and another client
// connection being provided to the application.
package birpc

import (
	"net"
	"net/rpc"

	"github.com/vsekhar/govtil/log"
	"github.com/vsekhar/govtil/net/server/direct"
	"github.com/vsekhar/govtil/net/muxconn"
)

func DispatchForever(connch <-chan net.Conn, srv *rpc.Server, clientch chan<- *rpc.Client) {
	for conn := range connch {
		muxed, err := muxconn.Split(conn, 2)
		if err != nil {
			log.Println("birpc: Failed to mux incoming connection from", conn.RemoteAddr().String(), "to", conn.LocalAddr().String(), ", dropping")
			continue
		}
		// Server on first muxed conn, client on second
		go srv.ServeConn(muxed[0])
		clientch <- rpc.NewClient(muxed[1])
	}
}

func Dial(url string, srv *rpc.Server) (client *rpc.Client, err error) {
	conn, err := direct.Dial(url)
	if err != nil { return }
	muxed, err := muxconn.Split(conn, 2)
	if err != nil { return }

	// Server on second, client on first (reverse of above)
	client = rpc.NewClient(muxed[0])
	go srv.ServeConn(muxed[1])
	return
}
