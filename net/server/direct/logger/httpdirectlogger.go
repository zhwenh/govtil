// httpdirectlogger.go connects to a host and logs all data returned by that
// host in a streamz format.
package main

import (
	"math/rand"
	"time"

	"github.com/vsekhar/govtil/log"
	"github.com/vsekhar/govtil/net/server/direct"
	"github.com/vsekhar/govtil/net/server/streamz"
)

var donech chan bool

func run() {
	defer func() {
		donech <- true
	}()

	conn, err := direct.Dial("http://localhost:8080/streamz")
	if err != nil {
		log.Fatal("failed to direct dial:", err)
	}
	ch := streamz.Channelify(conn)
	timeout := time.After(time.Duration(rand.Intn(10)) * time.Second)
	for s := range ch {
		select {
		case <-timeout:
			return
		default:
		}
		log.Print(s.Key, "=", s.Value)
	}
}

func main() {
	n := 100
	donech = make(chan bool)
	for i := 0; i < n; i++ {
		go run()
	}
	for i := 0; i < n; i++ {
		<-donech
	}
}
