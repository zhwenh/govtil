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
	"github.com/vsekhar/govtil/net/server/streamz"
	"github.com/vsekhar/govtil/net/server/direct"
)

// TODO: testing using net/http/httptest

var Healthz = healthz.NewHandler()
var Varz = varz.NewHandler()
var DirectCh = make(chan net.Conn)
var StreamzCh = make(chan []byte, 50)

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
	defer func() {
		if vnet.SocketClosed(err) {
			err = nil
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandler)
	mux.Handle("/healthz", logginghandler.New(Healthz, log.DEBUG))
	mux.Handle("/varz", logginghandler.New(Varz, log.DEBUG))

	sub := make(chan net.Conn)
	mux.Handle("/streamz", &direct.Handler{sub})
	streamz.Start(sub, StreamzCh)
	go streamz.Ticker(StreamzCh)

	addr := ":" + fmt.Sprint(port)
	l, err := vnet.Listen("tcp", addr, os.Interrupt)
	if err != nil { return }

	srv := &http.Server{Addr: addr, Handler: mux}
	err = srv.Serve(l)
	return
}
