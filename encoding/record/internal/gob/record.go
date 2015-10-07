package gob

// This file is part of the implementation of govtil/encoding/record, but
// resides in this internal fork of encoding/gob in order to access and/or
// export non-exported members of encoding/gob.

import (
	"fmt"
	"reflect"
)

// getBaseType returns the type stripped of its pointer(s) and returns the base
// type and the number of indirections. If the type provided is not a pointer,
// getBaseType returns the same type and 0 for indirections.
func getBaseType(t reflect.Type) (reflect.Type, int) {
	// Inspired by go1.5.1/encoding/gob/type.go:validUserType()
	slowpoke := t // walks half as fast as t
	fastpoke := t
	n := 0
	for  {
		if fastpoke.Kind() != reflect.Ptr {
			break
		}
		fastpoke = fastpoke.Elem()
		if fastpoke == slowpoke {
			panic("cannot use recursive pointer type " + t.String())
		}
		if n%2 == 0 {
			slowpoke = slowpoke.Elem()
		}
		n++
	}
	return fastpoke, n
}

func (enc *Encoder) checkForType(e interface{}) bool {
	t, _ := getBaseType(reflect.TypeOf(e))
	g, ok := enc.types[t]
	if !ok {
		return false
	}
	if _, ok := enc.idToType[g.id()]; !ok {
		return false
	}
	m := enc.typeInfoMap.Load().(map[reflect.Type]*typeInfo)
	if _, ok := m[t]; !ok {
		return false
	}
	return true
}

// StrictEncode is like Encode, but returns an error if the type has not been
// previously registered with the Encoder.
func (enc *Encoder) StrictEncode(e interface{}) error {
	return enc.StrictEncodeValue(reflect.ValueOf(e))
}

func (enc *Encoder) StrictEncodeValue(e reflect.Value) error {
	if !enc.checkForType(e.Interface()) {
		return fmt.Errorf("type not found: %s", reflect.TypeOf(e).Name())
	}
	return enc.EncodeValue(e)
}
