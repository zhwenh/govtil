package gob

import (
	"bytes"
	"reflect"
	"testing"
)

type unregisteredT struct {
	Dontregisterme int
}

type t1 struct {
	I int
}

func TestRecordCheckForType(t *testing.T) {
	enc := NewEncoder(nil)
	v := new(t1)
	enc.Register(v)
	if enc.checkForType(v) == false {
		t.Errorf("type not found after Register-ing: %s", reflect.TypeOf(v).Name())
	}
	if enc.checkForType(new(unregisteredT)) == true {
		t.Error("type found without Register-ing")
	}
}

type t2 struct {
	J int
	S t1
}

func TestRecordCheckForTypeNested(t *testing.T) {
	enc := NewEncoder(nil)
	v := new(t2)
	enc.Register(v)
	if enc.checkForType(v) == false {
		t.Errorf("type not found after Register-ing: %s", reflect.TypeOf(v).Name())
	}
	nv := new(t1)
	if enc.checkForType(nv) == true {
		t.Error("nested type found without Register-ing")
	}
}

func TestStrictEncode(t *testing.T) {
	b := new(bytes.Buffer)
	enc := NewEncoder(b)
	s1 := new(t1)
	enc.Register(s1)
	s1.I = 7
	if err := enc.StrictEncode(s1); err != nil {
		t.Errorf("error encoding registered type: %s", err)
	}
	if enc.StrictEncode(new(unregisteredT)) == nil {
		t.Errorf("no error when StrictEncod-ing unregistered type")
	}

	dec := NewDecoder(b)
	r1 := new(t1)
	if err := dec.Decode(&r1); err != nil {
		t.Errorf("error decoding: %s", err)
	}
	if *r1 != *s1 {
		t.Error("values do not match")
	}
}

func TestStrictEncodeNested(t *testing.T) {
	b := new(bytes.Buffer)
	enc := NewEncoder(b)
	s2 := new(t2)
	s2.J = 9
	s2.S.I = 10
	enc.Register(s2)
	if err := enc.StrictEncode(s2); err != nil {
		t.Errorf("error encoding registered type: %s", err)
	}
	if enc.StrictEncode(new(unregisteredT)) == nil {
		t.Errorf("no error when StrictEncod-ing unregistered type")
	}

	dec := NewDecoder(b)
	r2 := new(t2)
	if err := dec.Decode(&r2); err != nil {
		t.Errorf("error decoding: %s", err)
	}
	if *r2 != *s2 {
		t.Error("values do not match")
	}
}
