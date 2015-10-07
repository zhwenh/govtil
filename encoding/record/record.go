// 
package record

import (
	"fmt"
	"io"
	"reflect"

	"github.com/vsekhar/govtil/encoding/record/internal/gob"
)

type encoder struct {
	genc   *gob.Encoder
	rtype  reflect.Type
}

// NewEncoder creates a new encoder that serializes to stream w for values of
// the same type as e.
func NewEncoder(w io.Writer, e interface{}) *encoder {
	enc := new(encoder)
	enc.genc = gob.NewEncoder(w)
	t := reflect.TypeOf(e)
	fmt.Printf("registering: %s", t.Name())
	enc.rtype = t
	enc.genc.Register(e)
	return enc
}

// Encode serializes the given value. The parameter's type must match the type
// used when creating the Encoder, otherwise an error is returned and no data is
// written to the stream.
func (enc *encoder) Encode(e interface{}) error {
	// types must match exactly
	if reflect.TypeOf(e) != enc.rtype {
		return fmt.Errorf("type mismatch: %s and %s", reflect.TypeOf(e).Name(), enc.rtype.Name())
	}
	return enc.genc.StrictEncode(e)
}

// Decoding a record does not perform any typechecking. This is a "fail-open"
// approach. If a value was properly written, we do our best to decode it,
// trusting that any type constraints were properly applied elsewhere in the
// system.

type decoder struct {
	gob.Decoder
}

func NewDecoder(r io.Reader) *decoder {
	return &decoder{*gob.NewDecoder(r)}
}
