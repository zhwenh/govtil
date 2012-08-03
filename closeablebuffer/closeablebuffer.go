package closeablebuffer

import (
	"bytes"
	"io"
	"sync"
)

// A closeable bounded multi-threaded buffer
type CloseableBuffer struct {
	buf *bytes.Buffer	// don't use embedding, since I don't want to refine
						// every Read*() and Write*() method, just the basic ones
	closed bool
	mutex *sync.Mutex
	full sync.Cond
	empty sync.Cond
	maxlen int
}

func New(m int) *CloseableBuffer {
	mutex := sync.Mutex{}
	return &CloseableBuffer{buf: &bytes.Buffer{},
							closed: false,
							mutex: &mutex,
							full: *sync.NewCond(&mutex),
							empty: *sync.NewCond(&mutex),
							maxlen: m}
}

// After Close() is called, Read() and Write() will fail with err == io.EOF
func (cb *CloseableBuffer) Close() error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.closed = true
	cb.buf.Reset()
	return nil
}

func (cb *CloseableBuffer) Closed() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	return cb.closed
}

// Refined Read() does exactly one of the following:
// 	1) gets data if there is any in the buffer
//	2) returns (0, io.EOF) if buffer is closed
//	3) waits for data if there isn't any, then tries again
func (cb *CloseableBuffer) Read(data []byte) (n int, err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	for {
		n, err = cb.buf.Read(data)
		if err == nil {
			cb.full.Signal()	// 1) data (no longer full)
			break
		} else if cb.closed {
			return 0, io.EOF	// 2) no more, io.EOF
		} else {
			cb.empty.Wait()		// 3) wait for more
		}
	}
	return
}

// Refined Write() does exactly one of the following:
//  1) writes data if there is room in the buffer
//  2) returns (0, io.EOF) if buffer is closed
//  3) waits for reads to make room if buffer is full
func (cb *CloseableBuffer) Write(data []byte) (n int, err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	if cb.closed {
		return 0, io.EOF
	}
	for len(data) > 0 {
		// Wait for not-full
		for cb.buf.Len() >= cb.maxlen {
			cb.full.Wait()
		}

		// Write what you can
		writeable := cb.maxlen - cb.buf.Len()
		if writeable > len(data) {
			writeable = len(data)
		}
		rn, err := cb.buf.Write(data[:writeable])
		n += rn
		if err != nil {
			return n, err
		}
		data = data[rn:]
		cb.empty.Signal()
	}
	return
}

func (cb *CloseableBuffer) Len() int {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	return cb.buf.Len()
}

func (cb *CloseableBuffer) Cap() int {
	return cb.maxlen - cb.buf.Len()
}
