/*
	Package dlog provides a debug logger. You can pepper your code with
	dlog.Print[f|ln](...) and dlog.Fatal[f|ln](...) calls. These will only
	result in actual logging if dlog.SetDebug(true) is first called.
*/
package dlog

import (
	"log"
	"sync"
)

var sb struct {
	bool
	sync.Mutex
}

func set(b bool) {
	sb.Lock()
	defer sb.Unlock()
	sb.bool = b
}

func test() bool {
	sb.Lock()
	defer sb.Unlock()
	return sb.bool
}

func SetDebug(b bool) {
	set(b)
}

func Print(args ...interface{}) { if test() { log.Print(args...) } }
func Printf(s string, args ...interface{}) { if test() { log.Printf(s, args...) } }
func Println(args ...interface{}) { if test() { log.Println(args...) } }

func Fatal(args ...interface{}) { if test() { log.Fatal(args...) } }
func Fatalf(s string, args ...interface{}) { if test() { log.Fatalf(s, args...) } }
func Fatalln(args ...interface{}) { if test() { log.Fatalln(args...) } }
