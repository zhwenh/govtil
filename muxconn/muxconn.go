// Muxer

package muxconn

import (
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

const defaultBufSize = 4096

type sendPacket struct {
	chno    uint
	payload []byte
	err     chan error
}

type muxConn struct {
	conn		net.Conn
	chno		uint
	writeErrCh	chan error
	readPipe	*io.PipeReader
	closewaiter	*sync.WaitGroup
}

func (mc *muxConn) Write(data []byte) (n int, err error) {
	defer func() {
		if socketClosed(err) {
			err = io.EOF
		}
	}()

	enc := gob.NewEncoder(mc.conn)
	for len(data) > 0 {
		func() {
			// always put something back for other writers
			defer func() {
				mc.writeErrCh <- err
			}()
			
			// get a token to claim access to the underlying connection
			err = <-mc.writeErrCh
			if err != nil {
				// short circuit if we've seen an error on this connection
				data = nil // stop loop
				return
			}
			
			// write at most a fixed amount (additional data will have to wait
			// for the next loop iteration)
			to_write := defaultBufSize
			if len(data) < to_write {
				to_write = len(data)
			}
			if err = enc.Encode(mc.chno); err != nil {
				return
			}
			if err = enc.Encode(to_write); err != nil {
				return
			}
			if err = enc.Encode(data[:to_write]); err != nil {
				return
			}
			n += to_write
			data = data[to_write:]
		}()
	}
	return
}

func (mc *muxConn) Read(data []byte) (n int, err error) {
	defer func() {
		if socketClosed(err) {
			err = io.EOF
		}
	}()

	n, err = mc.readPipe.Read(data)
	return
}

func (mc *muxConn) Close() error {
	mc.closewaiter.Done()
	return nil
}

func (mc *muxConn) LocalAddr() net.Addr {
	return mc.conn.LocalAddr()
}

func (mc *muxConn) RemoteAddr() net.Addr {
	return mc.conn.RemoteAddr()
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
	if err == nil {
		return false
	}
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
func Split(conn net.Conn, n int) (muxconns []net.Conn, err error) {
	if n <= 0 {
		err = errors.New("Invalid number of connections to split into: " + fmt.Sprint(n))
		return
	}
	
	writeErrCh := make(chan error, 1)	// buffered, acts as a mutex
	writeErrCh <- nil					// prime with the first token
	pipeReaders := make([]*io.PipeReader, n)
	pipeWriters := make([]*io.PipeWriter, n)
	for i := 0; i < n; i++ {
		pipeReaders[i], pipeWriters[i] = io.Pipe()
	}
	var closewaiter sync.WaitGroup
	closewaiter.Add(n)
	
	// closer
	go func() {
		closewaiter.Wait()
		conn.Close()
	}()
	
	// read pump
	go func() {
		var err error = nil
		defer func() {
			for _,writer := range pipeWriters {
				writer.CloseWithError(err)
			}
		}()
		
		buffer := make([]byte, defaultBufSize)
		dec := gob.NewDecoder(conn)
		for {
			var chno uint
			err = dec.Decode(&chno)
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
			if plen > defaultBufSize {
				log.Fatal("muxconn: packet too large:", plen, "( max:", defaultBufSize, ")")
			}
			sub_buffer := buffer[:plen]
			err = dec.Decode(&sub_buffer)
			if err != nil {
				return
			}
			pipeWriters[chno].Write(sub_buffer)
		}
	}()

	muxconns = make([]net.Conn, n)	
	for i := 0; i < n; i++ {
		muxconns[i] = &muxConn{conn, uint(i), writeErrCh, pipeReaders[i], &closewaiter}
	}
	return
}
