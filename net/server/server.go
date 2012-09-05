// Package server provides a generic process server with healthz, varz and
// direct socket functionality
package server

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/vsekhar/govtil/log"
	vnet "github.com/vsekhar/govtil/net"
	"github.com/vsekhar/govtil/net/server/healthz"
	"github.com/vsekhar/govtil/net/server/logginghandler"
	"github.com/vsekhar/govtil/net/server/varz"
	"github.com/vsekhar/govtil/net/server/direct"
)

// TODO: testing using net/http/httptest

var Healthz = healthz.NewHandler()
var Varz = varz.NewHandler()
var DirectCh = make(chan net.Conn)

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

// placeholder request handler
func defaultHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>govtil/server %s!</h1>", r.URL.Path[1:])
}

// Serve on a given port
//
// The server will log to the default logger and will gracefully terminate on
// receipt of an os.Interrupt.
//
func ServeForever(port int) (err error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandler)
	mux.Handle("/healthz", Healthz)
	mux.Handle("/varz", Varz)
	mux.Handle("/direct", &direct.Handler{DirectCh})

	addr := ":" + fmt.Sprint(port)
	l, err := vnet.Listen("tcp", addr, os.Interrupt) // closed on SIGINT
	if err != nil { return }

	lh, err := logginghandler.New(mux, log.DEBUG)
	if err != nil { return }
	srv := &http.Server{Addr: addr, Handler: lh}
	err = srv.Serve(l)
	if vnet.SocketClosed(err) {
		err = nil
	}
	return
}
