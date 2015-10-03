package record

import (
	"bytes"
	"testing"
	"encoding/gob"
)

type s1 struct {
	I int
}

type s2 struct {
	A byte
	D string
	S s1
}


var values = []interface{}{
	15000,
	-14000,
	[]int{7, 42},
	&s2{'a', "hello", s1{4}},
}

func TestGob(t *testing.T) {
	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	for _, v := range values {
		if err := enc.Encode(v); err != nil {
			t.Fatalf("encoding '%v': %v", v, err)
		}
	}
	t.Logf("bytes: %v", b.Bytes())

	buf := make([]byte, 256)
	for b.Len() > 0 {
		size, _, err := decodeUintReader(b, buf)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("size: %d", size)

		typeid, tsize, err := decodeIntReader(b, buf)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("type id: %d", typeid)
		remaining := size - uint64(tsize)
		if _, err = b.Read(buf[0:remaining]); err != nil {
			t.Fatal(err)
		}
		t.Logf("  remaining bytes: %v", buf[0:remaining])
	}
}

