// Package healthz provides a simple healthz implementation
package healthz

import (
	"net/http"

	"github.com/vsekhar/govtil/log"
)

// A function that returns a health status as a bool (true == OK)
type HealthzFunc func() bool

type subHealthzHandler struct {
	HealthzFunc
	string
}

type healthzHandler struct {
	handlers []*subHealthzHandler
}

// Create a new healthz handler, an http.Handler that aggregates healthz
// responses from a number of registered HealthzFunc's
func NewHandler() *healthzHandler {
	return &healthzHandler{}
}

// Register a HealthzFunc to be polled when a healthz request is received
func (hh *healthzHandler) Register(hf HealthzFunc, name string) {
	hh.handlers = append(hh.handlers, &subHealthzHandler{hf, name})
}

// Serve an HTTP request (do not call this, it is exported so net/http can
// access it)
func (hh *healthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	repch := make(chan bool) 
	for _, handler := range hh.handlers {
		go func(h *subHealthzHandler) {
			healthy := h.HealthzFunc()
			if healthy {
				repch <- healthy
			} else {
				log.Println("healthz failed:", h.string)
			}
		}(handler)
	}
	ret := true
	for _ = range hh.handlers {
		if !<-repch {
			ret = false
		}
	}
	if ret {
		w.Write([]byte("OK\n"))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
