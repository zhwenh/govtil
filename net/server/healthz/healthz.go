package healthz

import (
	"log"
	"net/http"
)

// A function that returns a health status
type HealthzFunc func() bool

type subHealthzHandler struct {
	HealthzFunc
	string
}

// HealthzHandler is an http.Handler that aggregates healthz values from any
// number of registered HealthzFunc's
type HealthzHandler struct {
	handlers []*subHealthzHandler
}

// Register a VarzFunc to be included in varz output
func (hh *HealthzHandler) Register(hf HealthzFunc, name string) {
	hh.handlers = append(hh.handlers, &subHealthzHandler{hf, name})
}

func (hh *HealthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	repch := make(chan bool) 
	for _, handler := range hh.handlers {
		go func(h *subHealthzHandler) {
			healthy := h.HealthzFunc()
			if !healthy {
				log.Println("healthz failed:", h.string)
			}
			repch <- healthy
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
