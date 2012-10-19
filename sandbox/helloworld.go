package main

import (
	"fmt"
	"os"
	_ "unsafe"
)

func main() {
	_, err := os.Stat("/")
	fmt.Println(err)
	fmt.Printf("Hello world\n")
	fmt.Printf("GOROOT=%s\n", os.Getenv("GOROOT"))
}
