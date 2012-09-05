// Package multihandler provides an aggregating http.Handler for registering
// multiple sources and collecting information from them in response to an
// HTTP request. It is primarily used for varz services.
package multihandler

import (
	"bytes"
	"net/http"
	"sync"
)

// MultiHandler aggregates the output of a group of http.Handler's.
//
// Each handler is given a separate in-memory http.ResponseWriter, as well as
// the original request. The data returned from all handlers is concatenated
// together in unspecified order. If any handler returns a non-ok status code
// (i.e. handlerStatus != http.StatusOK), then one of the non-ok status codes
// is arbitrarily chosen and returned as the status code for the original
// response along with any data written.
//
// A zero-value MultiHandler is one that returns empty responses with StatusOK.
//
type MultiHandler struct {
	sync.RWMutex
	handlers []http.Handler
}

type subResponseWriter struct {
	buf    bytes.Buffer
	status int
}

func (srw *subResponseWriter) Header() (hdr http.Header) {
	return
}

func (srw *subResponseWriter) Write(b []byte) (int, error) {
	return srw.buf.Write(b)
}

func (srw *subResponseWriter) WriteHeader(i int) {
	srw.status = i
}

// Handle a request (called by the HTTP server)
func (mh *MultiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mh.RLock()
	defer mh.RUnlock()
	replies := make(chan *subResponseWriter)
	for _, handler := range mh.handlers {
		go func(handler http.Handler) {
			srw := subResponseWriter{}
			handler.ServeHTTP(&srw, r)
			replies <- &srw
		}(handler)
	}

	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	status := 0
	for _ = range mh.handlers {
		reply := <-replies
		reply.buf.WriteTo(buf)
		if reply.status != http.StatusOK {
			status = reply.status
		}
	}
	if status != 0 {
		w.WriteHeader(status)
	}
	buf.WriteTo(w)
}

// Register a handler to be included in this MultiHandler
func (mh *MultiHandler) Register(handler http.Handler) {
	mh.Lock()
	defer mh.Unlock()
	mh.handlers = append(mh.handlers, handler)
}
