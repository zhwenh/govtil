/*
Package sandbox provides a sandboxed implementation of the go compiler and
library. It can be used to execute untrusted code in an isolated manner.

Functions that may expose system vulnerabilites are short-circuited to return
errors. This means that untrusted code that imports and uses system resources
WILL compile, but WON'T run. This is to allow for code that may check if it can
perform a task but otherwise is able to do something useful if it cannot.

Some examples of functions that are removed from the sandboxed standard library
are:
	os.Stat()
	os.Open()
	unsafe.*  // everything
*/
package sandbox
