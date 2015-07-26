package birpc

import (
	"errors"
	"fmt"
	"net/http"
	"net/rpc"
	"testing"

//	"golang.org/x/net/websocket"

	"github.com/vsekhar/govtil/log"
	vnet "github.com/vsekhar/govtil/net"
//	"github.com/vsekhar/govtil/net/multiplex"
)

func init() {
	log.SetVerbosity(log.DEBUG)
}

type Args struct {
	A, B int
}

type Quotient struct {
	Quo, Rem int
}

type Product struct {
	R int
}

type Arith struct {}

func (t *Arith) Multiply(args *Args, prod *Product) error {
	prod.R = args.A * args.B
	return nil
}

func (t *Arith) Divide(args *Args, quo *Quotient) error {
	if args.B == 0 {
		return errors.New("divide by zero")
	}
	quo.Quo = args.A / args.B
	quo.Rem = args.A % args.B
	return nil
}

func TestBiRPC(t *testing.T) {
	// setup RPC
	arith := new(Arith)
	srv := rpc.NewServer()
	srv.Register(arith)
	clientch := make(chan *rpc.Client)

	http.Handle("/birpc", HTTPHandleFunc(srv, clientch))

	// Start server
	const port = 11235
	l, err := vnet.SignalListener(port)
	if err != nil {
		t.Fatalf("Server: %v", err)
	}
	go http.Serve(l, http.DefaultServeMux)

	// Dial to self
	dialClient, err := Dial("ws://localhost:"+fmt.Sprint(port)+"/birpc", srv)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	sch := <- clientch // server's client

	// call from dialers side
	prod := Product{}
	err = dialClient.Call("Arith.Multiply", &Args{2,4}, &prod)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if prod.R != 8 {
		t.Errorf("bad prod '%d', expected %d", prod.R, 8)
	}

	// call from server's side
	quot := Quotient{}
	err = sch.Call("Arith.Divide", &Args{16,2}, &quot)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if quot.Quo != 8 || quot.Rem != 0 {
		t.Errorf("bad quot %v", quot)
	}
}