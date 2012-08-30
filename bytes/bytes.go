package bytes

import (
	"math/rand"
)

func RandBytes(n int) []byte {
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		ret[i] = byte(rand.Int31n(256))
	}
	return ret
}

func Equals(b1, b2 []byte) bool {
	if len(b1) != len(b2) {
		return false
	}
	for i, cb := range b1 {
		if cb != b2[i] {
			return false
		}
	}
	return true
}

func Reverse(a []byte) {
	// reverse a slice in place
	for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
		a[i], a[j] = a[j], a[i]
	}
}
