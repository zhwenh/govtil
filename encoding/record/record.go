// Package record provides fast dynamic record-based serialization.
//
// Serializing go objects results in a payload of records and a record spec.
// The record spec can itself be saved and used to encode subsequent records.
// The record spec is used to decode and varify payloads.
package record

import (
	"bytes"
	"io"
	"encoding/gob"  // implemented as a wrapper around gob
)

type Encoder struct {
	w io.Writer
	b bytes.Buffer
	ge *gob.Encoder
}

func NewEncoder(w io.Writer) *Encoder {
	ret := &Encoder{}
	ret.w = w
	ret.ge = gob.NewEncoder(&ret.b)
	return ret
}

func (enc *Encoder) Encode(e interface{}) error {
	return nil
}