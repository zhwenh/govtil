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
	"os/signal"
	"syscall"

	"github.com/vsekhar/govtil/log"
	"github.com/vsekhar/govtil/mem"
	vnet "github.com/vsekhar/govtil/net"
	"github.com/vsekhar/govtil/net/server/birpc"
	"github.com/vsekhar/govtil/net/server/borkborkbork"
	"github.com/vsekhar/govtil/net/server/direct"
	"github.com/vsekhar/govtil/net/server/healthz"
	"github.com/vsekhar/govtil/net/server/logginghandler"
	"github.com/vsekhar/govtil/net/server/streamz"
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

	// birpc
	birpcconns := make(chan net.Conn)
	http.Handle("/birpc", &direct.Handler{birpcconns})
	go birpc.DispatchForever(birpcconns, RPC, RPCClientsCh)

	// streamz
	subs := make(chan net.Conn)
	http.Handle("/streamz", &direct.Handler{subs})
	go streamz.DispatchForever(subs, StreamzCh)
	go streamz.Ticker(StreamzCh)

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
func ServeForever(port int) error {
	addr := ":" + fmt.Sprint(port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Errorln("govtil/net/server: Failed to listen on", port, err)
		return err
	}

	// Close listen port on signals (causes http.Serve() to return)
	sigch := make(chan os.Signal)
	signal.Notify(sigch, []os.Signal{
		syscall.SIGABRT,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGPWR,
		syscall.SIGQUIT,
		syscall.SIGSTOP,
		syscall.SIGTERM,
	}...)
	go func() {
		sig := <-sigch
		log.Println("govtil/net/server: Closing listen port", l.Addr().String(), "due to signal", sig)
		l.Close()
	}()

	logginghandler := logginghandler.New(http.DefaultServeMux, log.NORMAL)
	err = http.Serve(l, logginghandler)
	if err != nil {
		if vnet.SocketClosed(err) {
			err = nil // closed due to signal, no error
		} else {
			log.Errorln("govtil/net/server:", err)
		}
	}
	log.Println("govtil/net/server: Terminating")
	return err
}
