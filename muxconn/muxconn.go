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

	"github.com/vsekhar/govtil/pipes"
)

const defaultBufSize = 4096

type sendPacket struct {
	chno    uint
	payload []byte
	err     chan error
}

type muxConn struct {
	connCh      chan net.Conn
	chno        uint
	writeErr    *error
	readPipe    *io.PipeReader
	closewaiter *sync.WaitGroup
}

func (mc *muxConn) Write(data []byte) (n int, err error) {
	defer func() {
		if socketClosed(err) {
			err = io.EOF
		}
	}()

	for len(data) > 0 {
		// make one attempt to write at most defaultBufSize bytes to the
		// underlying connection
		func() {
			// claim channel (acts as a mutex)
			conn := <-mc.connCh
			defer func() { mc.connCh <- conn }()

			// short circuit if we've seen an error on this connection.
			if *mc.writeErr != nil {
				data = nil // stop loop
				err = *mc.writeErr
				return
			}

			// update based on any errors seen on this write attempt
			defer func() { *mc.writeErr = err }()

			enc := gob.NewEncoder(conn)

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
	conn := <-mc.connCh
	defer func() {
		mc.connCh <- conn
	}()
	return conn.LocalAddr()
}

func (mc *muxConn) RemoteAddr() net.Addr {
	conn := <-mc.connCh
	defer func() {
		mc.connCh <- conn
	}()
	return conn.RemoteAddr()
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
	// TODO: update this with additional (perhaps non-TCP) checks
	// TODO: replace this with a check for net.errClosing when/if it's public
	errString := err.Error()
	if err == io.EOF ||
		strings.HasSuffix(errString, "use of closed network connection") ||
		strings.HasSuffix(errString, "broken pipe") ||
		strings.HasSuffix(errString, "connection reset by peer") {
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
//
// Muxed connections are not buffered, therefore if data arrives on the
// underlying connection for channel 0, that data must be read by muxconn[0]
// before any other data will be read from the underlying connection. Muxed
// connections should only be used when the read rates across muxed channels do
// not vastly differ, in order ensure fast channels are not starved waiting for
// slow ones.
func Split(conn net.Conn, n int) (muxconns []net.Conn, err error) {
	return doSplit(conn, n, io.Pipe)
}

// SplitBuffered is the same as Split, but uses buffered pipes internally. Use
// this version only if you expect the different muxed channels to be read at
// very different rates, otherwise Split is likely faster
func SplitBuffered(conn net.Conn, n int) (muxconns []net.Conn, err error) {
	return doSplit(conn, n, pipes.Buffered)
}

func doSplit(conn net.Conn, n int, makepipe func() (*io.PipeReader, *io.PipeWriter)) (muxconns []net.Conn, err error) {
	if n <= 0 {
		err = errors.New("Invalid number of connections to split into: " + fmt.Sprint(n))
		return
	}

	// The underlying connection lives in a buffered channel and is pulled out
	// for each goroutine that wants to write data (reads are handled by the
	// read pump below)
	connCh := make(chan net.Conn, 1)
	connCh <- conn

	// A goroutine waits for all proxies to Close(), and then closes the
	// underlying Conn
	var closewaiter sync.WaitGroup
	closewaiter.Add(n)
	go func() {
		closewaiter.Wait()
		// claim channel and put back so Write()'s will properly err out
		conn := <-connCh
		defer func() {
			connCh <- conn
		}()
		conn.Close()
	}()

	// A read pump keeps reading frames from the underlying Conn and writing
	// them to the correct pipe
	pipeReaders := make([]*io.PipeReader, n)
	pipeWriters := make([]*io.PipeWriter, n)
	for i := 0; i < n; i++ {
		pipeReaders[i], pipeWriters[i] = makepipe()
	}
	go func() {
		var err error = nil
		defer func() {
			for _, writer := range pipeWriters {
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

	// The proxies and their methods expose a net.Conn interface for each muxed
	// channel
	muxconns = make([]net.Conn, n)
	var writeErr error
	for i := 0; i < n; i++ {
		muxconns[i] = &muxConn{connCh: connCh, chno: uint(i),
			writeErr: &writeErr, readPipe: pipeReaders[i],
			closewaiter: &closewaiter}
	}
	return
}
