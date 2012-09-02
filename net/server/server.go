// Package server provides a generic process server with healthz and varz
// functionality
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
)

var Healthz = healthz.NewHandler()
var Varz = varz.NewHandler()

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
	// Create all of this instead of using http.ListenAndServe() so as not to
	// pollute http package variables, and to control the server (for signals,
	// etc.)

	mux := newServeMux()
	mux.HandleFunc("/", defaultHandler)
	mux.Handle("/healthz", Healthz)
	mux.Handle("/varz", Varz)

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
