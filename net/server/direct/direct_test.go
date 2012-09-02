package direct

import (
	"log"
	"net"
	"net/http"
	"testing"

	vbytes "github.com/vsekhar/govtil/bytes"
)

func TestDirect(t *testing.T) {
	// setup HTTP server with a directhandler
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal("Could not set up listen: ", err)
	}
	defer listener.Close()
	connch := make(chan net.Conn)
	srv := &http.Server{Handler: &Handler{connch}}
	go srv.Serve(listener)
	
	indata := []byte("abc123")
	outdata := []byte("123abcd")

	// Handle inbound connections: read then write
	go func() {
		for conn := range connch {
			rdata := make([]byte, len(indata))
			n, err := conn.Read(rdata)
			if err != nil {
				t.Error("failed to read")
				conn.Close()
				continue
			}
			if !vbytes.Equals(rdata[:n], indata) {
				t.Error("data mismatch:", rdata, indata)
			}
			n, err = conn.Write(outdata)
			if err != nil {
				t.Error("failed to send")
			}
			conn.Close()
		}
	}()
	
	// establish outbound connection
	addr := listener.Addr().String()
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal("Couldn't parse address:", addr)
	}
	url := "http://:" + p
	log.Println(url)
	conn, err := Dial(url)
	if err != nil {
		t.Fatal("Could not direct connect:", err)
	}
	defer conn.Close()

	n, err := conn.Write(indata)
	if err != nil || n != len(indata) {
		t.Error("couldn't write:", err, n)
	}

	rdata := make([]byte, len(outdata))
	n, err = conn.Read(rdata)
	if err != nil {
		t.Fatal("Read failed")
	}
	if !vbytes.Equals(rdata[:n], outdata) {
		t.Error("data mismatch:", rdata[:n], outdata)
	}
}
