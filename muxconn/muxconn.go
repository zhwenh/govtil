// Muxer

// Also try implementing a Codec that does this (but only 2 way, not
// as general as n-way muxing here). The codec would implement both the
// ServerCodec and ClientCodec interfaces in the same object.

package muxconn

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type sendPacket struct {
	chno    uint
	payload []byte
	err     chan error
}

type wirePacket struct {
	Chno    uint
	Payload []byte
}

type muxConn struct {
	chno     uint
	localaddr net.Addr
	remoteaddr net.Addr
	sendch   chan sendPacket
	closewaiter *sync.WaitGroup
	recvbuf  *bytes.Buffer
	recvcond *sync.Cond
}

func (cp *muxConn) Write(data []byte) (n int, err error) {
	errch := make(chan error)
	cp.sendch <- sendPacket{cp.chno, data, errch}
	err = <-errch
	if err == nil {
		n = len(data)
	}
	return
}

func (cp *muxConn) Read(data []byte) (n int, err error) {
	cp.recvcond.L.Lock()
	defer cp.recvcond.L.Unlock()
	if cp.recvbuf.Len() == 0 {
		cp.recvcond.Wait()
	}
	n, err = cp.recvbuf.Read(data)
	return
}

func (cp *muxConn) Close() error {
	cp.closewaiter.Done()
	return nil
}

func (cp *muxConn) LocalAddr() net.Addr {
	return cp.localaddr
}

func (cp *muxConn) RemoteAddr() net.Addr {
	return cp.remoteaddr
}

func (*muxConn) SetDeadline(time.Time) error {
	return errors.New("muxConn does not implement deadlines")
}

func (*muxConn) SetReadDeadline(time.Time) error {
	return errors.New("muxConn does not implement deadlines")
}

func (*muxConn) SetWriteDeadline(time.Time) error {
	return errors.New("muxConn does not implement deadlines")
}

// MuxConn muxes a network connection into 'n' separate connections. It returns
// a slice of 'n' connection proxies and an error.
//
// The connection proxies satisfy the net.Conn interface and can be used in
// place of the underlying single connection. When all connection proxies
// operating on a single connection are closed, the underlying connection is
// closed.
//
// Muxed connections match up in index order. If two ends of a connection are
// muxed, the muxconn[0] on one end matches up with muxconn[0] on the other.
// If one end is muxed to N proxies and another to M, then the number of valid
// muxed connections is min(N,M). If a send occurs on muxed connection k where
// min(N,M) < k <= max(N,M), the receiving end will log.Fatal(...)
func Split(conn net.Conn, n int) (muxconns []*muxConn, err error) {
	if n <= 0 {
		err = errors.New("Invalid number of connections to split into: " + fmt.Sprint(n))
		return
	}
	sendch := make(chan sendPacket)
	gob.Register(wirePacket{})

	// Closer
	var closewaiter sync.WaitGroup
	closewaiter.Add(n)
	go func() {
		closewaiter.Wait()
		close(sendch) // signal to send pump
	}()

	// Send pump
	go func() {
		enc := gob.NewEncoder(conn)
		for {
			sp, ok := <-sendch
			if !ok { // signal from closer
				conn.Close()
				return
			}
			err := enc.Encode(wirePacket{sp.chno, sp.payload})
			if err != nil {
				log.Fatal("error encoding: ", err)
				sp.err <- err
			} else {
				sp.err <- nil
			}
		}
	}()

	// Raw receive byte channels
	recvrawch := make([]chan []byte, n)
	for i := 0; i < n; i++ {
		recvrawch[i] = make(chan []byte)
	}

	// Receive pump
	go func(conn net.Conn, recvrawch []chan []byte) {
		dec := gob.NewDecoder(conn)
		for {
			var wp wirePacket
			err := dec.Decode(&wp)
			if err != nil {
				// TODO: replace this with a check for net.errClosing when/if
				// it becomes public
				if err == io.EOF ||
					strings.HasSuffix(err.Error(), "use of closed network connection") {
					// close all channels
					for _, ch := range recvrawch {
						close(ch)
					}
					return
				} else {
					log.Fatal("receive decoding error: ", err)
				}
			}
			if int(wp.Chno) > n {
				// invalid channel number
				log.Fatal("gogp/peers/muxconn: Dropping wirePacket for invalid mux channel ", wp.Chno) 
			}

			recvrawch[wp.Chno] <- wp.Payload
		}
	}(conn, recvrawch)

	// Buffers
	buffers := make([]bytes.Buffer, n)
	conditions := make([]*sync.Cond, n)

	// Buffer pumps
	for i := 0; i < n; i++ {
		conditions[i] = sync.NewCond(&sync.Mutex{})
		go func(recvrawch chan []byte, buffer *bytes.Buffer, condition *sync.Cond) {
			for {
				data, ok := <-recvrawch

				// If ok, write to buffer, else shut down buffer pump
				// (either way Signal() to waiting Read()s)
				writer := func() bool {
					condition.L.Lock()
					defer condition.L.Unlock()
					if ok {
						buffer.Write(data)
					}
					condition.Signal()
					return ok
				}

				// If writer fails, channel is closed, so we're done
				if !writer() {
					break
				}
			}
		}(recvrawch[i], &buffers[i], conditions[i])
	}

	// muxConn proxies
	muxconns = make([]*muxConn, n)
	for i := 0; i < n; i++ {
		muxconns[i] = &muxConn{chno: uint(i), localaddr: conn.LocalAddr(),
			remoteaddr: conn.RemoteAddr(), sendch: sendch,
			closewaiter: &closewaiter, recvbuf: &buffers[i],
			recvcond: conditions[i]}
	}

	return muxconns, nil
}
