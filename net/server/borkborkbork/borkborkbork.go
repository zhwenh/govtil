package borkborkbork

import (
	"fmt"
	"net/http"
	"os"
	"time"
	
	"github.com/vsekhar/govtil/log"
)

type borkBorkBorkHandler struct {
	sig os.Signal
}

// convenience function
func New(sig os.Signal) *borkBorkBorkHandler {
	return &borkBorkBorkHandler{sig}
}

var Delay = 1 * time.Second

func (bbbh *borkBorkBorkHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pid := os.Getpid()
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		log.Error("borkborkbork: Failed to find my process", pid, err)
		http.Error(w, "borkborkbork: Failed to get my process", http.StatusInternalServerError)
		return
	}
	msg := fmt.Sprintf("OK (waiting %s)", Delay)
	w.Write([]byte(msg))
	go func() {
		time.Sleep(Delay)
		log.Alwaysln("borkborkbork: sending myself", bbbh.sig.String())
		proc.Signal(bbbh.sig)
	}()
}
