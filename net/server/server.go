// Package server provides a generic process server with healthz, varz and
// direct socket functionality
package server

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/rpc"
	"os"

	"github.com/vsekhar/govtil/log"
	vnet "github.com/vsekhar/govtil/net"
	"github.com/vsekhar/govtil/net/server/birpc"
	"github.com/vsekhar/govtil/net/server/direct"
	"github.com/vsekhar/govtil/net/server/healthz"
	"github.com/vsekhar/govtil/net/server/logginghandler"
	"github.com/vsekhar/govtil/net/server/streamz"
	"github.com/vsekhar/govtil/net/server/varz"
)

// TODO: testing using net/http/httptest

var Healthz = healthz.NewHandler()
var Varz = varz.NewHandler()
var DirectCh = make(chan net.Conn)
var StreamzCh = make(chan []byte, 50)
var RPC = rpc.NewServer()
var RPCClientsCh = make(chan *rpc.Client, 50)

// Register a function providing healthz information. Function must be of the
// form:
//	func myHealthzFunc() bool {...}
//
func RegisterHealthz(f healthz.HealthzFunc, name string) {
	Healthz.Register(f, name)
}

// Register a function that writes varz information. Function must be of the
// form:
//	func myVarzFunc(io.Writer) error {...}
//
func RegisterVarz(f varz.VarzFunc, name string) {
	Varz.Register(f, name)
}

func RegisterRPC(rcvr interface{}) error {
	return RPC.Register(rcvr)
}

func RegisterRPCName(name string, rcvr interface{}) error {
	return RPC.RegisterName(name, rcvr)
}

// placeholder request handler
func defaultHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>govtil/net/server %s!</h1>", r.URL.Path[1:])
}

// Serve on a given port
//
// The server will log to the default logger and will gracefully terminate on
// receipt of an os.Interrupt.
//
// The following URLs are defined:
//    /
//    /healthz
//    /varz
//    /streamz
//    /birpc
//    /debug/pprof
//
func ServeForever(port int) (err error) {
	defer func() {
		if vnet.SocketClosed(err) {
			err = nil
		}
	}()

	http.Handle("/", logginghandler.New(http.HandlerFunc(defaultHandler), log.NORMAL))
	http.Handle("/healthz", logginghandler.New(Healthz, log.NORMAL))
	http.Handle("/varz", logginghandler.New(Varz, log.NORMAL))

	// birpc
	birpcconns := make(chan net.Conn)
	http.Handle("/birpc", logginghandler.New(&direct.Handler{birpcconns}, log.NORMAL))
	go birpc.DispatchForever(birpcconns, RPC, RPCClientsCh)

	// streamz
	subs := make(chan net.Conn)
	http.Handle("/streamz", &direct.Handler{subs})
	go streamz.DispatchForever(subs, StreamzCh)
	go streamz.Ticker(StreamzCh)

	addr := ":" + fmt.Sprint(port)
	l, err := vnet.Listen("tcp", addr, os.Interrupt)
	if err != nil {
		log.Println("govtil/net/server: Failed to listen on", port, err)
		return
	}

	err = http.Serve(l, nil)
	if err != nil {
		log.Println("govtil/net/server: Server terminated with error", err)
	}
	return
}
