// Muxer

// Also try implementing a Codec that does this (but only 2 way, not
// as general as n-way muxing here). The codec would implement both the
// ServerCodec and ClientCodec interfaces in the same object.

package muxconn

import (
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/vsekhar/govtil/closeablebuffer"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	MAX_BUFFER_SIZE = 8192 // bytes
)

type sendPacket struct {
	chno    uint
	payload []byte
	err     chan error
}

type muxConn struct {
	chno     uint
	localaddr net.Addr
	remoteaddr net.Addr
	sendch   chan sendPacket
	closewaiter *sync.WaitGroup
	recvbuf  io.ReadWriteCloser
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
	return cp.recvbuf.Read(data)
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

// Return whether the given error indicates a socket that produced it has been
// closed by the other end
func socketClosed(err error) bool {
	// TODO: update this with additional checks
	// TODO: replace this with a check for net.errClosing when/if it's public
	if err == io.EOF ||
		strings.HasSuffix(err.Error(), "use of closed network connection") {
		return true
	}
	return false
}

// Split muxes a network connection into 'n' separate connections. It returns
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

	// Closer
	var closewaiter sync.WaitGroup
	closewaiter.Add(n)
	go func() {
		closewaiter.Wait()
		close(sendch) // signal to send pump
	}()

	// Send pump
	go func() {
		defer conn.Close()
		enc := gob.NewEncoder(conn)
		var err error = nil
		for {
			sp, ok := <-sendch
			if !ok { return } // closed, no more

			if err != nil {
				// already got a socket send error, so reply with that same
				// error
				sp.err <- err
				continue
			}

			// Send in order:
			//  1. Channel number
			//  2. Payload length
			//  3. Payload
			if err = enc.Encode(sp.chno); err != nil {
				sp.err <- err
			} else if err = enc.Encode(len(sp.payload)); err != nil {
				sp.err <- err
			} else if err = enc.Encode(sp.payload); err != nil {
				sp.err <- err
			} else {
				sp.err <- nil
			}
		}
	}()
	
	// Receive buffers
	recvbufs := make([]io.ReadWriteCloser, n)
	for i := 0; i < n; i++ {
		recvbufs[i] = closeablebuffer.New(MAX_BUFFER_SIZE)
	}

	// Receive pump
	go func() {
		// Close all receiving buffers to signal to readers that no more is coming
		defer func() {
			for _, buf := range recvbufs {
				buf.Close()
			}
		}()

		dec := gob.NewDecoder(conn)
		for {
			// Receive in order:
			//  1. Channel number
			//  2. Payload length
			//  3. Payload
			var chno uint
			err := dec.Decode(&chno)
			if err != nil {
				return
			}
			if int(chno) >= n {
				log.Fatal("muxconn: Receive got invalid mux channel ", chno) 
			}
			var plen int
			err = dec.Decode(&plen)
			if err != nil {
				return
			}
			payload := make([]byte, plen)
			err = dec.Decode(&payload)
			if err != nil {
				return
			}
			recvbufs[chno].Write(payload)
		}
		return
	}()

	// muxConn proxies
	muxconns = make([]*muxConn, n)
	for i := 0; i < n; i++ {
		muxconns[i] = &muxConn{chno: uint(i), localaddr: conn.LocalAddr(),
			remoteaddr: conn.RemoteAddr(), sendch: sendch,
			closewaiter: &closewaiter, recvbuf: recvbufs[i]}
	}

	return muxconns, nil
}
