package muxconn

import (
	"bytes"
//	"govtil/dlog"
	"encoding/gob"
	"errors"
	"io"
	"log"
	"net"
	"net/rpc"
	"testing"
)

// Set up a connection to myself (for testing)
func SelfConnection() (net.Conn, net.Conn) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("Could not set up listen: ", err)
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

func MuxPairs(inconn, outconn net.Conn, n int) (ins []net.Conn, outs []net.Conn, err error) {
	if inconn != nil {
		ins, err = Split(inconn, n)
		if err != nil { return }
	}
	if outconn != nil {
		outs, err = Split(outconn, n)
		if err != nil { return }
	}
	return
}

func TestConnection(t *testing.T) {
	inconn, outconn := SelfConnection()
	defer inconn.Close()
	defer outconn.Close()
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
	defer outconn.Close()

	// Use inconn as a sender
	inchannels, err := Split(inconn, 2)
	if err != nil {
		t.Error("Split failed: ", err)
	}
	in := inchannels[1]
	defer inchannels[0].Close()

	sdata := []byte("hello")
	go func() {
		in.Write(sdata)
		in.Close()
	}()

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
	err = dec.Decode(&rdata)
	if err != nil {
		t.Error("Split conn rdata failed")
	}
	if !bytes.Equal(rdata, sdata) {
		t.Error("Split send failed: ", sdata, " != ", rdata)
	}
}

func TestSplitReceiver(t *testing.T) {
	inconn, outconn := SelfConnection()
	outchannels, err := Split(outconn, 2)
	if err != nil {
		t.Error("Split failed: ", err)
	}
	out := outchannels[1]

	chno := uint(1)
	sdata := []byte("hello")
	sdatalen := len(sdata)
	enc := gob.NewEncoder(inconn)
	go func() {
		err := enc.Encode(chno)
		if err != nil {
			t.Error("Split conn write chno failed")
		}
		err = enc.Encode(sdatalen)
		if err != nil {
			t.Error("Split conn write sdatalen failed")
		}
		err = enc.Encode(sdata)
		if err != nil {
			t.Error("Split conn write sdata failed")
		}
	}()
	rdata := make([]byte, len(sdata))
	n, err := out.Read(rdata)
	if n != len(sdata) || err != nil || !bytes.Equal(rdata, sdata) {
		t.Error("Split receive failed: ", rdata)
	}
}

func TestClose(t *testing.T) {
	inconn, outconn := SelfConnection()
	ins, outs, err := MuxPairs(inconn, outconn, 2)
	if err != nil {
		t.Fatal("MuxPairs failed: ", err)
	}

	// Close one mux, should be able to read from the other
	ins[0].Close()
	sdata := []byte{11,23,5}
	go func() {
		ins[1].Write(sdata)
	}()
	rdata := make([]byte, 3)
	outs[1].Read(rdata)
	if !bytes.Equal(sdata, rdata) {
		t.Error("Half-closed connection: bytes don't match")
	}

	// Close other mux, reads should return io.EOF
	ins[1].Close()
	_, err0 := ins[0].Read(rdata)
	_, err1 := ins[1].Read(rdata)
	if err0 != io.EOF || err1 != io.EOF {
		t.Error("Bad error codes on closed muxed conn:", err0, err1)
	}
}

// Stress test with many muxes
func TestNMuxes(t *testing.T) {
	var n int
	if testing.Short() {
		n = 100
	} else {
		n = 10000
	}

	inconn, outconn := SelfConnection()
	ins, outs, err := MuxPairs(inconn, outconn, n)
	if err != nil {
		t.Fatal("MuxPairs failed: ", err)
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

// Spawn RPC servers and return clients
func SetupRPC(ins, outs []net.Conn) (ret []*rpc.Client, err error) {
	if len(ins) != len(outs) {
		err = errors.New("len(ins) and len(outs) must match")
		return
	}
	recv := new(RPCRecv)
	for _,in := range ins {
		srv := rpc.NewServer()
		srv.Register(recv)
		go srv.ServeConn(in)
	}
	for _,out := range outs {
		ret = append(ret, rpc.NewClient(out))
	}
	return ret, nil
}

func TestRPC(t *testing.T) {
	inconn, outconn := SelfConnection()
	ins, outs, err := MuxPairs(inconn, outconn, 2)
	if err != nil {
		t.Error("MuxPairs failed: ", err)
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
	inconn, outconn := SelfConnection()
	ins, outs, err := MuxPairs(inconn, outconn, 2)
	if err != nil {
		t.Error("MuxPairs failed: ", err)
	}

	type pair struct {
		In net.Conn
		Out net.Conn
	}

	pairs := make([]pair, 2)
	pairs[0].In = ins[0]
	pairs[1].In = ins[1]
	pairs[0].Out = outs[0]
	pairs[1].Out = outs[1]

	for _,p := range pairs {
		if p.In.LocalAddr().String() != p.Out.RemoteAddr().String() {
			t.Error("Address mismatch: ", p.In.LocalAddr(), " != ", p.Out.RemoteAddr())
		}
		if p.In.RemoteAddr().String() != p.Out.LocalAddr().String() {
			t.Error("Address mismatch: ", p.In.RemoteAddr(), " != ", p.Out.LocalAddr())
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

func TestRPCDropClientConn(t *testing.T) {
	inconn, outconn := SelfConnection()
	ins, outs, err := MuxPairs(inconn, outconn, 2)
	if err != nil {
		t.Fatal("MuxPairs failed: ", err)
	}

	srv := rpc.NewServer()
	srv.Register(new(RPCRecv))
	go srv.ServeConn(ins[0])
	client := rpc.NewClient(outs[0])
	sdata := "abc"
	rdata := ""
	err = client.Call("RPCRecv.Echo", &sdata, &rdata)
	if err != nil {
		t.Error("Regular RPC call failed: ", err)
	}

	outconn.Close()
	err = client.Call("RPCRecv.Echo", &sdata, &rdata)
	if err == nil {
		t.Error("RPC call on closed MuxConn client did not fail")
	}
}

func TestRPCDropServerConn(t *testing.T) {
	inconn, outconn := SelfConnection()
	ins, outs, err := MuxPairs(inconn, outconn, 2)
	if err != nil {
		t.Error("MuxPairs failed: ", err)
	}

	srv := rpc.NewServer()
	srv.Register(new(RPCRecv))
	go srv.ServeConn(ins[0])
	client := rpc.NewClient(outs[0])
	sdata := "abc"
	rdata := ""
	err = client.Call("RPCRecv.Echo", &sdata, &rdata)
	if err != nil {
		t.Error("Regular RPC call failed: ", err)
	}

	inconn.Close()
	err = client.Call("RPCRecv.Echo", &sdata, &rdata)
	if err == nil {
		t.Error("RPC call on closed MuxConn server did not fail")
	}
}

// Stress buffers and flow control
func TestRPCBigData(t *testing.T) {
	var plen int
	if testing.Short() {
		plen = 1024				// 1 kB
	} else {
		plen = 10 * 1024 * 1024	// 10 MB
	}

	payloadbytes := make([]byte, plen)
	for i := 0; i < len(payloadbytes); i++ {
		payloadbytes[i] = byte(i)
	}
	payload := string(payloadbytes)

	inconn, outconn := SelfConnection()
	ins, outs, err := MuxPairs(inconn, outconn, 2)
	if err != nil {
		t.Fatal("MuxPairs failed: ", err)
	}
	clients, _ := SetupRPC(ins, outs)
	success := make(chan bool)
	for _,client := range clients {
		go func() {
			rpayload := ""
			client.Call("RPCRecv.Echo", &payload, &rpayload)
			success <- rpayload == payload
		}()
	}
	for _ = range clients {
		if !<-success {
			t.Fatal("Bigdata failed")
		}
	}
}
