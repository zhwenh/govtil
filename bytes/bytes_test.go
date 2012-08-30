package bytes

import (
	"testing"
)

func TestEquals(t *testing.T) {
	b := RandBytes(100)
	if !Equals(b, b) {
		t.Error("equals failed")
	}
}

func TestReverse(t *testing.T) {
	b := []byte{1, 6, 3, 4, 8, 5, 3}
	rb := []byte{3, 5, 8, 4, 3, 6, 1}
	Reverse(b)
	if !Equals(b, rb) {
		t.Error("reverse failed")
	}
}
