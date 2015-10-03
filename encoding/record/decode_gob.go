package record

// This file contains routines for minimally decoding a gob stream.

import (
	"errors"
	"io"
)

// FROM go1.5.1/encoding/gob/decode.go
const uint64Size = 8

// FROM go1.5.1/encoding/gob/decode.go (modified)
var (
	errBadUint = errors.New("record: encoded unsigned integer out of range")
)


// FROM go1.5.1/encoding/gob/decode.go
// decodeUintReader reads an encoded unsigned integer from an io.Reader.
// Used only by the Decoder to read the message length.
func decodeUintReader(r io.Reader, buf []byte) (x uint64, width int, err error) {
        width = 1
        n, err := io.ReadFull(r, buf[0:width])
        if n == 0 {
                return
        }
        b := buf[0]
        if b <= 0x7f {
                return uint64(b), width, nil
        }
        n = -int(int8(b))
        if n > uint64Size {
                err = errBadUint
                return
        }
        width, err = io.ReadFull(r, buf[0:n])
        if err != nil {
                if err == io.EOF {
                        err = io.ErrUnexpectedEOF
                }
                return
        }
        // Could check that the high byte is zero but it's not worth it.
        for _, b := range buf[0:width] {
                x = x<<8 | uint64(b)
        }
        width++ // +1 for length byte
        return
}

// decodeIntReader reads an encoded signed integer from io.Reader.
// Used to read type ids.
func decodeIntReader(r io.Reader, buf []byte) (x int64, width int, err error) {
	ux, width, err := decodeUintReader(r, buf)
	if err != nil {
		return
	}
	complement := (ux & 0x01 == 1)
	ux = ux >> 1
	if complement {
		ux = ^ux
	}
	return int64(ux), width, nil
}

func advanceN(r io.Reader, n int, buf []byte) (done int, err error) {
	if done, err = r.Read(buf[0:n]); err != nil {
		return done, err
	}
	return n, nil
}

