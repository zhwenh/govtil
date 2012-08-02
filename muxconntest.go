package govtil

import (
	"bytes"
	"encoding/gob"
	"log"
	"net"
	"net/rpc"
	"testing"
)

// Set up a connection to myself (for testing)
func SelfConnection() (net.Conn, net.Conn) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Println(err)
		log.Fatal("Could not set up listen")
	}
	defer listener.Close()

	inconnch := make(chan net.Conn)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("Couldn't receive connection")
		}
		inconnch <- conn
	}()

	outconn, _ := net.Dial("tcp", listener.Addr().String())
	inconn := <-inconnch
	return inconn, outconn
}

func TestConnection(t *testing.T) {
	// test connection
	inconn, outconn := SelfConnection()
	data := make([]byte, 2)
	data[0] = 0
	data[1] = 1
	outconn.Write(data)
	rdata := make([]byte, 2)
	inconn.Read(rdata)
	if data[0] != rdata[0] || data[1] != rdata[1] {
		t.Error("Basic socket comms failed")
	}

	enc := gob.NewEncoder(outconn)
	dec := gob.NewDecoder(inconn)
	sstr := "hello"
	go func() {
		enc.Encode(sstr)
	}()
	var rstr string
	dec.Decode(&rstr)
	if rstr != sstr {
		t.Error("Encoder over socket failed")
	}

}

func TestSplitSender(t *testing.T) {
	inconn, outconn := SelfConnection()
	inchannels, err := MuxConn(inconn, 2)
	if err != nil {
		t.Error("Split failed: ", err)
	}
	in := inchannels[1]
	defer in.Close()

	sdata := []byte("hello")
	in.Write(sdata)

	dec := gob.NewDecoder(outconn)
	var rchno uint
	var rdatalen int
	err = dec.Decode(&rchno)
	if err != nil || rchno != 1 {
		t.Error("Split conn chno failed")
	}
	err = dec.Decode(&rdatalen)
	if err != nil || rdatalen != len(sdata) {
		t.Error("Split conn rdatalen failed")
	}
	rdata := make([]byte, rdatalen)
	n, err := outconn.Read(rdata)
	if err != nil || n != len(sdata) {
		t.Error("Split conn rdata failed")
	}
	if !bytes.Equal(rdata, sdata) {
		t.Error("Split send failed")
	}
}

func TestSplitReceiver(t *testing.T) {
	inconn, outconn := SelfConnection()
	outchannels, err := MuxConn(outconn, 2)
	if err != nil {
		t.Error("Split failed: ", err)
	}
	out := outchannels[1]

	chno := uint(1)
	sdata := []byte("hello")
	sdatalen := len(sdata)
	enc := gob.NewEncoder(inconn)
	err = enc.Encode(&chno)
	if err != nil {
		t.Error("Split conn write chno failed")
	}
	err = enc.Encode(&sdatalen)
	if err != nil {
		t.Error("Split conn write sdatalen failed")
	}
	n, err := inconn.Write(sdata)
	if err != nil || n != len(sdata) {
		t.Error("Split conn write sdata failed")
	}
	rdata := make([]byte, len(sdata))
	n, err = out.Read(rdata)
	if n != len(sdata) || err != nil || !bytes.Equal(rdata, sdata) {
		t.Error("Split receive failed")
	}
}

// Stress test with many muxes
func TestNMuxes(t *testing.T) {
	const n = 10000

	inconn, outconn := SelfConnection()
	ins, err := MuxConn(inconn, n)
	if err != nil {
		t.Error("Split failed: ", err)
	}
	outs, err := MuxConn(outconn, n)
	if err != nil {
		t.Error("Split failed: ", err)
	}
	
	// in --> out
	sdata := []byte{11,23,5}
	go func() {
		for _,c := range ins {
			c.Write(sdata)
		}
	}()
	rch := make(chan []byte)
	for _,c := range outs {
		go func(c net.Conn) {
			rdata := make([]byte, len(sdata))
			c.Read(rdata)
			rch <- rdata
		}(c)
	}
	
	for i := 0; i < n; i++ {
		if !bytes.Equal(sdata, <-rch) {
			t.Error("Failed on channel ", i)
		}
	}
}

type RPCRecv int
func (r *RPCRecv) Echo(in *string, out *string) error {
	*out = *in
	return nil
}

func TestRPC(t *testing.T) {
	inconn, outconn := SelfConnection()
	ins, err := MuxConn(inconn, 2)
	if err != nil {
		t.Error("Split failed: ", err)
	}
	outs, err := MuxConn(outconn, 2)
	if err != nil {
		t.Error("Split failed: ", err)
	}
	recv := new(RPCRecv)
	rpc.Register(recv)
	go rpc.ServeConn(ins[0])
	client := rpc.NewClient(outs[0])
	sdata := "hello"
	rdata := ""
	err = client.Call("RPCRecv.Echo", &sdata, &rdata)
	if err != nil {
		t.Error("RPC call failed: ", err)
	}
	if sdata != rdata {
		t.Error("RPC Echo failed")
	}
}

func TestXRPC(t *testing.T) {
	var n int = 2
	inconn, outconn := SelfConnection()
	ins, err := MuxConn(inconn, n)
	if err != nil {
		t.Error("Split failed: ", err)
	}
	outs, err := MuxConn(outconn, n)
	if err != nil {
		t.Error("Split failed: ", err)
	}

	type pair struct {
		In net.Conn
		Out net.Conn
	}

	pairs := make([]pair, n)
	for i := 0; i < int(n); i++ {
		pairs[i].In = ins[i]
		pairs[i].Out = outs[i]
	}

	for _,p := range pairs {
		if p.In.LocalAddr().String() != p.Out.RemoteAddr().String() {
			t.Error("Address mismatch: ", p.In.LocalAddr(), " and ", p.Out.RemoteAddr())
		}
		if p.In.RemoteAddr().String() != p.Out.LocalAddr().String() {
			t.Error("Address mismatch: ", p.In.RemoteAddr(), " and ", p.Out.LocalAddr())
		}
	}

	srv := rpc.NewServer()
	srv.Register(new(RPCRecv))
	go srv.ServeConn(pairs[0].In)
	go srv.ServeConn(pairs[1].Out)
	client1 := rpc.NewClient(pairs[0].Out)
	defer client1.Close()
	client2 := rpc.NewClient(pairs[1].In)
	defer client2.Close()

	sdata1 := "abc"
	sdata2 := "123"
	rdata1 := ""
	rdata2 := ""

	call1 := client1.Go("RPCRecv.Echo", &sdata1, &rdata1, nil)
	call2 := client2.Go("RPCRecv.Echo", &sdata2, &rdata2, nil)
	<-call2.Done
	<-call1.Done
	if sdata1 != rdata1 || sdata2 != rdata2 {
		t.Error("XRPC failed")
	}
}
