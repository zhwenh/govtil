package direct

import (
	"net"
	"net/http"
	"net/rpc"
	"testing"

	vbytes "github.com/vsekhar/govtil/bytes"
	vtesting "github.com/vsekhar/govtil/testing"
)

func setupServer() (connch chan net.Conn, url string, closech chan bool, err error) {
	// setup HTTP server with a directhandler
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return
	}
	connch = make(chan net.Conn)
	closech = make(chan bool)
	go func() {
		<-closech
		listener.Close()
	}()
	srv := &http.Server{Handler: &Handler{connch}}
	go srv.Serve(listener)
	_, p, _ := net.SplitHostPort(listener.Addr().String())
	url = "http://:" + p
	return
}

func TestDirect(t *testing.T) {
	connch, url, closech, err := setupServer()
	
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
	conn, err := Dial(url)
	if err != nil {
		t.Fatal("Could not direct connect:", err)
	}
	defer conn.Close()

	// write some data
	n, err := conn.Write(indata)
	if err != nil || n != len(indata) {
		t.Error("couldn't write:", err, n)
	}

	// read some data
	rdata := make([]byte, len(outdata))
	n, err = conn.Read(rdata)
	if err != nil {
		t.Fatal("Read failed")
	}
	if !vbytes.Equals(rdata[:n], outdata) {
		t.Error("data mismatch:", rdata[:n], outdata)
	}
	closech <- true
}

func TestRPCDirect(t *testing.T) {
	connch, url, closech, _ := setupServer()

	// outbound connection is client
	conn, err := Dial(url)
	if err != nil {
		t.Fatal("Could not direct connect:", err)
	}
	client := rpc.NewClient(conn)

	// inbound connection is server
	srv := rpc.NewServer()
	srv.Register(new(vtesting.RPCRecv))
	go srv.ServeConn(<-connch)
	sdata := "9876"
	rdata := ""
	err = client.Call("RPCRecv.Echo", &sdata, &rdata)
	if err != nil || sdata != rdata {
		t.Error("RPCDirect failed:", err, sdata, rdata)
	}
	defer client.Close()
	closech <- true
}
