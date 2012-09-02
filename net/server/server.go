// Package server provides a generic process server with healthz, varz and
// direct socket functionality
package server

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"

	vnet "github.com/vsekhar/govtil/net"
	"github.com/vsekhar/govtil/net/server/healthz"
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

type serveMux struct {
	*http.ServeMux
}

// Add logging to http.ServeMux
func (mux *serveMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("govtil/net/server request from", r.RemoteAddr, "for", r.RequestURI)
	mux.ServeMux.ServeHTTP(w,r)
}

func newServeMux() *serveMux {
	return &serveMux{http.NewServeMux()}
}

// Serve on a given port
//
// The server log to the default logger and will gracefully terminate on receipt
// of an os.Interrupt.
//
func ServeForever(port int) {
	mux := newServeMux()
	mux.HandleFunc("/", defaultHandler)
	mux.Handle("/healthz", Healthz)
	mux.Handle("/varz", Varz)
	mux.Handle("/direct", &direct.Handler{DirectCh})

	// Create a listen socket that is closed upon os.Interrupt
	addr := ":" + fmt.Sprint(port)
	srv := &http.Server{Addr: addr, Handler: mux}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("Could not listen on address:", addr)
	}
	sigch := make(chan os.Signal)
	signal.Notify(sigch, os.Interrupt)
	go func() {
		<-sigch
		log.Println("Interrupt received, closing server")
		l.Close()
	}()
	err = srv.Serve(l)
	if !vnet.SocketClosed(err) {
		panic(err)
	}
}
