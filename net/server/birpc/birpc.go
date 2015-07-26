// Package birpc provides a bi-directional RPC handler. Incoming connections
// are muxed, with one connection going to the RPC server and another client
// connection being provided to the application.
package birpc

import (
	"net/rpc"

	"golang.org/x/net/websocket"

	"github.com/vsekhar/govtil/net/multiplex"
)

func HTTPHandleFunc(srv *rpc.Server, cch chan<- *rpc.Client) websocket.Handler {
	return websocket.Handler(func(c *websocket.Conn) {
		muxed := multiplex.Split(c, 2)
		go func() {
			cch <- rpc.NewClient(muxed[1])
		}()
		srv.ServeConn(muxed[0])
	})
}

// On the dialing side, Dial opens a socket to a listener, serves one end itself
// and returns the other end as an rpc.Client.
func Dial(url string, srv *rpc.Server) (client *rpc.Client, err error) {
	conn, err := websocket.Dial(url, "", "http://localhost")
	if err != nil {
		return nil, err
	}
	muxed := multiplex.Split(conn, 2)

	// Server on second, client on first (reverse of above)
	client = rpc.NewClient(muxed[0])
	go srv.ServeConn(muxed[1])
	return
}
