package multiplex

import (
	"bytes"
	"io"
	"testing"

	vio "github.com/vsekhar/govtil/io"
)

var msg0 []byte = []byte("abc123")
var msg1 []byte = []byte("def456")

var msgs [][]byte = [][]byte{msg0, msg1}

func SplitReadWriteCloser(rwc io.ReadWriteCloser, n uint) []io.ReadWriteCloser {
	rcs := SplitReadCloser(rwc, n)
	wcs := SplitWriteCloser(rwc, n)
	var r []io.ReadWriteCloser
	for i := 0; i < int(n); i++ {
		r = append(r, vio.NewReadWriteCloser(rcs[i], wcs[i]))
	}
	return r
}

// For test routines, runs data through split ReadWriteClosers
func DoTestBiDirectional(t *testing.T, rwcs0 []io.ReadWriteCloser, rwcs1 []io.ReadWriteCloser) {
	type entry struct {
		p []byte
		r io.Reader
		w io.Writer
	}

	if len(rwcs0) != len(rwcs1) {
		t.Fatalf("length mismatch: %v and %v", len(rwcs0), len(rwcs1))
	}
	var tests []entry
	for i, _ := range rwcs0 {
		tests = append(tests, entry{msg0, rwcs0[i], rwcs1[i]})
		tests = append(tests, entry{msg1, rwcs1[i], rwcs0[i]})
	}

	go func() {
		for _, e := range tests {
			e.w.Write(e.p)
		}
	}()

	for n, e := range tests {
		r := make([]byte, 100)
		rn, err := e.r.Read(r)
		r = r[:rn]
		if err != nil {
			t.Fatalf("failed to read for test %v", n)
		}
		if !bytes.Equal(e.p, r[:rn]) {
			t.Fatalf("bytes not equal: expected '%v', got '%v'", e.p, r)
		}
	}
}
