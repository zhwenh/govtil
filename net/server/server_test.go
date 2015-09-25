package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/vsekhar/govtil/log"
	vtest "github.com/vsekhar/govtil/testing"
)

func init() {
	log.SetVerbosity(log.DEBUG)
}

// TODO: factor out server start/stop (but in a way that allows for testing
// signal stop, borkborkbork, etc.)
func startServer(_ *testing.T) {

}

func TestServer(t *testing.T) {
	l, port, err := vtest.LocalListener()
	go func() {
		if ServeListenerForever(l) != nil {
			t.Fatal(err)
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
