package runtime

import (
	stdruntime "runtime"
)

var stackBuf = make([]byte, 4096)

// Get stack trace (don't use in panic situations as this function allocs)
func Stack() []byte {
	n := stdruntime.Stack(stackBuf, false)
	return stackBuf[:n]
}
