package bytes

import (
	sbytes "bytes"
	"testing"
)

func TestReverse(t *testing.T) {
	b := []byte{1, 6, 3, 4, 8, 5, 3}
	rb := []byte{3, 5, 8, 4, 3, 6, 1}
	Reverse(b)
	if !sbytes.Equal(b, rb) {
		t.Error("reverse failed")
	}
}
