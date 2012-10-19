/*
Package test is compiled using the sandbox compiler and library. This package
attempts each operation that is not permitted in the sandbox and fails its test
if any of these operations succeeds.
*/
package test

import (
	"fmt"
	"os"
	_ "unsafe"
	"testing"
)

func TestHelloWorld(t *testing.T) {
	_, err := os.Stat("/")
	fmt.Println(err)
	fmt.Printf("Hello world\n")
	fmt.Printf("GOROOT=%s\n", os.Getenv("GOROOT"))
	//t.Error("blah")
}
