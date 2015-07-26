// Package server provides a generic process server with healthz, varz and
// direct socket functionality
package server

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/rpc"
	"syscall"

	"github.com/vsekhar/govtil/log"
	"github.com/vsekhar/govtil/mem"
	vnet "github.com/vsekhar/govtil/net"
	"github.com/vsekhar/govtil/net/server/birpc"
	"github.com/vsekhar/govtil/net/server/borkborkbork"
	"github.com/vsekhar/govtil/net/server/healthz"
	"github.com/vsekhar/govtil/net/server/logginghandler"
	"github.com/vsekhar/govtil/net/server/varz"
)

// TODO: testing using net/http/httptest

// Healthz handler. Use Healthz.Register() to register a healthz function
var Healthz = healthz.NewHandler()

// Varz handler. Use Varz.Register() to register a varz function
var Varz = varz.NewHandler()

// StreamzCh is a chan []byte to which streamz values should be written. Use
// govtil/net/server/streamz.Write() to write to this channel on the server.
var StreamzCh = make(chan []byte, 50)

// RPC is an rpc.Server that handles connections received at the /birpc URL.
// Use RPC.Register() to register method receivers.
var RPC = rpc.NewServer()

// RPCClientCh is a chan *rpc.Client from which RPC clients should be read.
// These clients are produced by birpc from incoming connections.
var RPCClientsCh = make(chan *rpc.Client, 50)

// A placeholder root request handler
func defaultHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "govtil/net/server %s!", r.URL.Path[1:])
}

func init() {
	http.Handle("/", http.HandlerFunc(defaultHandler))
	http.Handle("/healthz", Healthz)
	http.Handle("/varz", Varz)
	http.Handle("/birpc", birpc.Handler(RPC, RPCClientsCh))

	killHandler := borkborkbork.New(syscall.SIGKILL)
	intHandler := borkborkbork.New(syscall.SIGINT)
	http.Handle("/killkillkill", killHandler)
	http.Handle("/intintint", intHandler)

	// mem
	Varz.Register(mem.Varz, "mem")
	http.HandleFunc("/create", mem.Create)
	http.HandleFunc("/delete", mem.Delete)
	http.HandleFunc("/gc", mem.GC)
}

// Serve on a given port
//
// The server will log to the default logger and will gracefully terminate on
// receipt of an interrupt or kill signal.
//
// The following URLs are defined:
//    /
//    /healthz
//    /varz
//    /streamz
//    /birpc
//    /debug/pprof
//
// Setting port to 0 will start the server on an ephemeral port. The assigned
// port will be logged.
func ServeForever(port int) error {
	l, err := vnet.SignalListener(port)
	if err != nil {
		return err
	}
	return serveListener(l)
}

// for testing
func serveListener(l net.Listener) error {
	// Wrap with logger
	handler := logginghandler.New(http.DefaultServeMux, log.GetVerbosity())

	_, aps, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		log.Errorf("govtil/net/server: failed to get port - %v", err)
	}
	log.Printf("govtil/net/server: starting on port %v", aps)

	// Serve
	err = http.Serve(l, handler)
	if err != nil {
		if vnet.SocketClosed(err) {
			err = nil // closed due to signal, no error
		} else {
			log.Errorf("govtil/net/server: %v", err)
		}
	}
	log.Println("govtil/net/server: Terminating")
	return err
}
