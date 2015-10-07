package record

import (
	"bytes"
	"testing"
)

type notRegistered struct {
	dontRegisterMe int
}

func TestEncoder(t *testing.T) {
	var i int
	b := new(bytes.Buffer)
	enc := NewEncoder(b, i)
	if err := enc.Encode(notRegistered{}); err == nil {
		t.Error("expected error encoding unregistered type")
	}
	if err := enc.Encode(i); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(notRegistered{}); err == nil {
		t.Error("expected error encoding unregistered type")
	}
}

func TestEncodeDecode(t *testing.T) {
	var i int = 7
	b := new(bytes.Buffer)
	enc := NewEncoder(b, i)
	if err := enc.Encode(i); err != nil {
		t.Fatal(err)
	}
	dec := NewDecoder(b)
	var ri int
	if err := dec.Decode(&ri); err != nil {
		t.Fatal(err)
	}
	if ri != i {
		t.Error("values do not match")
	}
}
