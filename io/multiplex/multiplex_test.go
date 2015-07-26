package multiplex

import (
	"bytes"
	"io"
	"encoding/gob"
	"testing"

	vio "github.com/vsekhar/govtil/io"
	"github.com/vsekhar/govtil/log"
)

func init() {
	log.SetVerbosity(log.DEBUG)
}

func TestSplitReadCloser(t *testing.T) {
	p1, p2 := io.Pipe()

	// write wire format
	go func() {
		enc := gob.NewEncoder(p2)
		for n, p := range msgs {
			if err := enc.Encode(uint(n)); err != nil {
				t.Fatalf("failed to write channel number: %v", err)
			}
			if err := enc.Encode(p); err != nil {
				t.Fatalf("failed to write payload: %v", err)
			}			
		}
		p2.Close()
	}()

	// read from channels
	rcs := SplitReadCloser(p1, 2)
	for n, p := range msgs {
		rmsg := make([]byte, len(p))
		if _, err := rcs[n].Read(rmsg); err != nil {
			t.Fatalf("failed to read on channel %v: %v", n, err)
		}
		if !bytes.Equal(rmsg, p) {
			t.Fatalf("bytes not equal: expected '%v', got '%v'", p, rmsg)
		}		
	}
}

func TestSplitWriteCloser(t *testing.T) {
	p1, p2 := io.Pipe()

	// write to channels
	go func() {
		wcs := SplitWriteCloser(p2, 2)
		for n, p := range msgs {
			wcs[n].Write(p)
		}
		for _, wc := range wcs {
			wc.Close()
		}
	}()

	// read wire format
	dec := gob.NewDecoder(p1)
	for n, p := range msgs {
		var chno uint
		var d []byte
		if err := dec.Decode(&chno); err != nil {
			t.Fatalf("could not decode channel no: %v", err)
		}
		if err := dec.Decode(&d); err != nil {
			t.Fatalf("could not decode payload on channel %v: %v", n, err)
		}
		if !bytes.Equal(d, p) {
			t.Fatal("bytes not equal: expected '%v', got '%v'", p, d)
		}
	}
}

func TestReadAndWrite(t *testing.T) {
	p1, p2 := io.Pipe()

	// write to channels
	go func() {
		wcs := SplitWriteCloser(p2, 2)
		for n, p := range msgs {
			wcs[n].Write(p)
		}
		for _, wc := range wcs {
			wc.Close()
		}
	}()

	// read from channels
	rcs := SplitReadCloser(p1, 2)
	for n, p := range msgs {
		r := make([]byte, 100)
		rn, err := rcs[n].Read(r)
		r = r[:rn]
		if ; err != nil {
			t.Fatalf("failed to read on channel %v: %v", n, err)
		}
		if !bytes.Equal(p, r) {
			t.Fatalf("bytes not equal: expected '%v', got '%v'", p, r)
		}		
	}
}

func TestBiDirectional(t *testing.T) {
	a0, a1 := io.Pipe()
	b0, b1 := io.Pipe()
	rwc0 := vio.NewReadWriteCloser(a0, b1)
	rwc1 := vio.NewReadWriteCloser(b0, a1)
	rwcs0 := SplitReadWriteCloser(rwc0, 2)
	rwcs1 := SplitReadWriteCloser(rwc1, 2)
	DoTestBiDirectional(t, rwcs0, rwcs1)
}

