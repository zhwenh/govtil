// Package direct provides an http.Handler and Dial'er for establishing a
// direct TCP connection to a server via its HTTP server. This allows arbitrary
// socket connections without needing to open additional ports on the server.
// It is primarily used by the streamz implementation.
package direct

import (
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	liburl "net/url"
	
	vbytes "github.com/vsekhar/govtil/bytes"
)

// Token exchange is used to make doubly sure a client from this package is
// talking to a server HTTP handler from this package. Otherwise we might be
// injecting raw socket connections into programs on the client or server side
// that are erroneously connected on the other end.

var inbounddirectOK = []byte("directOK11235813")
var outbounddirectOK = []byte("directOK3141592654")

func sendServerToken(conn net.Conn) (err error) {
	_, err = conn.Write(inbounddirectOK)
	return
}

func getServerToken(conn net.Conn) (err error) {
	rdata := make([]byte, len(inbounddirectOK))
	_, err = conn.Read(rdata)
	if err != nil { return err }
	if !vbytes.Equals(rdata, inbounddirectOK) {
		return errors.New("client: server token exchange failed")
	}
	return nil
}

func sendClientToken(conn net.Conn) (err error) {
	_, err = conn.Write(outbounddirectOK)
	return
}

func getClientToken(conn net.Conn) (err error) {
	rdata := make([]byte, len(outbounddirectOK))
	_, err = conn.Read(rdata)
	if err != nil { return err }
	if !vbytes.Equals(rdata, outbounddirectOK) {
		return errors.New("client: server token exchange failed")
	}
	return nil
}

// An http.Handler that accepts connections and pushes them onto a channel
type Handler struct {
	Chan chan net.Conn
}

// Handle incoming direct connection request, pushing underlying TCP connection
// to the handler's channel.
func (dh *Handler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "direct/Handler.ServeHTTP: ResponseWriter provided does not support Hijack()", http.StatusInternalServerError)
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, "Could not hijack direct connection, " + err.Error(), http.StatusInternalServerError)
		return
	}

	err = sendServerToken(conn)
	if err != nil {
		http.Error(w, "server/directhandler: sending server token, " + err.Error(), http.StatusInternalServerError)
		return
	}
	err = getClientToken(conn)
	if err != nil {
		http.Error(w, "server/directhandler: getting client token, " + err.Error(), http.StatusInternalServerError)
		return
	}
	if dh.Chan != nil {
		dh.Chan <- conn
	} else {
		http.Error(w, "Direct connection request dropped, no handler channel", http.StatusInternalServerError)
	}
}

func DialAddr(addr, path string) (conn net.Conn, err error) {
	url := "http://" + addr + path
	return Dial(url)
}

// Establish a direct TCP connection to a server via its HTTP server. URL must
// point to an endpoint on the server that is handled by Handler.
//
// Example:
//
//	handler := &direct.Handler{make(chan net.Conn)}
//	http.Handle("/direct", handler)
//	http.ListenAndServe(":8080", nil)
//
//	// accept incoming direct connections from http server
//	go func() {
//		for conn := range handler.Chan {
//			conn.Write([]byte("abc123")
//			conn.Close()
//		}
//	}()
//
//	// initiate a direct connection to http server
//	conn, _ := direct.Dial("http://:8080/direct")
//	rdata := make([]byte, 6)
//	conn.Read(rdata) // == "abc123"
//	conn.Close()
//
func Dial(url string) (conn net.Conn, err error) {
	// Get hostname and dial
	parsedurl, err := liburl.Parse(url)
	if err != nil { return }
	conn, err = net.Dial("tcp", parsedurl.Host)
	if err != nil {	return }

	// Issue an HTTP request with the provided url (which should be configured
	// with a Handler)
	clientconn := httputil.NewClientConn(conn, nil)
	r, err := http.NewRequest("GET", url, nil)
	if err != nil { return }
	err = clientconn.Write(r)
	if err != nil {	return }

	// Hijack and return underlying connection and verify an OK response
	conn, _ = clientconn.Hijack()
	err = getServerToken(conn)
	if err != nil {
		err = errors.New("server/directdial: getting server token, " + err.Error())
		return
	}
	err = sendClientToken(conn)
	if err != nil {
		err = errors.New("server/directdial: sending client token, " + err.Error())
		return
	}
	return
}
