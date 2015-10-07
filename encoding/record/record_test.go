package record

import (
	"bytes"
	"testing"
)

type notRegistered struct {
	dontRegisterMe int
}

type t1 struct {
	I int
}

func TestEncoder(t *testing.T) {
	s1 := new(t1)
	s1.I = 7
	b := new(bytes.Buffer)
	enc := NewEncoder(b, s1)
	if err := enc.Encode(notRegistered{}); err == nil {
		t.Error("expected error encoding unregistered type")
	}
	if err := enc.Encode(s1); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(notRegistered{}); err == nil {
		t.Error("expected error encoding unregistered type")
	}
}

func TestEncodeDecode(t *testing.T) {
	s1 := new(t1)
	s1.I = 9
	b := new(bytes.Buffer)
	enc := NewEncoder(b, s1)
	if err := enc.Encode(s1); err != nil {
		t.Fatal(err)
	}
	dec := NewDecoder(b)
	r1 := new(t1)
	if err := dec.Decode(&r1); err != nil {
		t.Fatal(err)
	}
	if *r1 != *s1 {
		t.Error("values do not match")
	}
}
