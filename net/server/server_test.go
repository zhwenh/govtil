package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/vsekhar/govtil/log"
	vnet "github.com/vsekhar/govtil/net"
)

func init() {
	log.SetVerbosity(log.DEBUG)
}

// TODO: factor out server start/top (but in a way that allows for testing
// signal stop, borkborkbork, etc.)
func startServer(_ *testing.T) {

}

func TestServer(t *testing.T) {
	l, err := vnet.SignalListener(0)
	if err != nil {
		t.Fatalf("failed to listen")
	}
	_, aps, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("failed to get port: %v", err)
	}
	port, err := strconv.Atoi(aps)
	if err != nil {
		t.Fatalf("failed to convert port string: %v, %v", aps, err)
	}

	go func() {
		if serveListener(l) != nil {
			t.Errorf("serveListener error: %v", err)
		}
	}()
	resp, err := http.Get("http://localhost:" + fmt.Sprintf("%d", port) + "/healthz")
	if err != nil {
		t.Fatalf("failed to get healthz: %v", err)
	}
	health, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	OK := []byte("OK\n")
	if !bytes.Equal(health, OK) {
		t.Errorf("failed health check, expected '%s', got '%s'", OK, health)
	}
}
