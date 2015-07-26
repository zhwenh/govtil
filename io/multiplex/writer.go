package multiplex

import (
	"encoding/gob"
	"io"
	"sync"

	"github.com/vsekhar/govtil/log"
)

type wop struct {
	id int
	b []byte
	c chan rp
}

type rp struct {
	n int
	e error
}

type wproxy struct {
	id int
	wc chan wop
	wr chan rp
	cg *sync.WaitGroup
}

func (p *wproxy) Write(d []byte) (int, error) {
	p.wc <- wop{p.id, d, p.wr}
	rp := <- p.wr
	return rp.n, rp.e
}

func (p *wproxy) Close() error {
	p.cg.Done()
	return nil
}

// Split a WriteCloser into 'n' WriteClosers. When all returned WriteClosers
// have been Close()'d, then the underlying ReadCloser is also closed.
func SplitWriteCloser(wc io.WriteCloser, n uint) []io.WriteCloser {
	var r []io.WriteCloser
	c := make(chan wop)
	cg := new(sync.WaitGroup)
	cg.Add(int(n))
	for i := 0; i < int(n); i++ {
		wpx := &wproxy{
			i,
			c,
			make(chan rp),
			cg,
		}
		r = append(r, wpx)
	}

	// Closer
	go func() {
		cg.Wait()
		wc.Close()
	}()

	// Write pump
	wpump := func() {
		var err error
		defer func() {
			if err != nil && err != io.EOF {
				log.Errorf("govtil/io/multiplex: wpump error: %v", err)
			}
		}()

		enc := gob.NewEncoder(wc)
		for {
			w, ok := <- c
			if !ok {
				err = nil
				return
			}
			if err = enc.Encode(uint(w.id)); err != nil {
				w.c <- rp{0, err}
				err = nil
				continue
			}
			if err = enc.Encode(w.b); err != nil {
				w.c <- rp{0, err}
				err = nil
				continue
			}
			w.c <- rp{len(w.b), nil}
		}
	}

	go wpump()
	return r
}
