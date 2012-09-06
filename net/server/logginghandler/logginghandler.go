// Package logginghandler provides an HTTP handler wrapper that performs simple
// logging for each HTTP request received, and then forwards the request to an
// underlying handler.
//
// Format of logging is:
//	<logger_timestamp> HTTP(<request_GUID>) <METHOD> from <RemoteAddr> for <URL>
//	<logger_timestamp> HTTP(<request_GUID>) writing <n> bytes
//
// Example:
//	2012/09/04 20:23:01 HTTP(fd2c2ad4) GET from 34.168.234.66:31839 for /path/to/something
//	2012/09/04 20:23:01 HTTP(fd2c2ad4) writing 25 bytes
//
package logginghandler

import (
	"net/http"
	
	"github.com/vsekhar/govtil/guid"
	"github.com/vsekhar/govtil/log"
)

type loggingResponseWriter struct {
	guid guid.GUID
	loglevel log.Level
	http.ResponseWriter
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	log.Logf(lrw.loglevel, "HTTP(%s) writing %d bytes", lrw.guid.Short(), len(b))
	return lrw.ResponseWriter.Write(b)
}

func (lrw *loggingResponseWriter) WriteHeader(i int) {
	log.Logf(lrw.loglevel, "HTTP(%s) writing header %d", lrw.guid.Short(), i)
	lrw.ResponseWriter.WriteHeader(i)
}

type loggingHandler struct {
	loglevel log.Level
	http.Handler
}

// Called by http.Server
func (lh *loggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g, err := guid.V4()
	if err != nil {
		http.Error(w, "couldn't create request GUID", http.StatusInternalServerError)
		return
	}
	rw := &loggingResponseWriter{g, lh.loglevel, w}
	log.Logf(lh.loglevel, "HTTP(%s) %s from %s for %s", g.Short(), r.Method, r.RemoteAddr, r.URL)
	lh.Handler.ServeHTTP(rw,r)
}

// Create a new logging handler and log at the specified level (see govtil/log
// for explanations of the log levels)
func New(h http.Handler, loglevel log.Level) (lh http.Handler) {
	return &loggingHandler{loglevel, h}
}
