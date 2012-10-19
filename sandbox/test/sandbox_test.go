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
