package multiplex

import (
	"bufio"
	"encoding/gob"
	"io"
	"sync"

	"github.com/vsekhar/govtil/log"
)

type rproxy struct {
	id int
	rp *io.PipeReader
	brp *bufio.Reader
	cg *sync.WaitGroup
}

func (p *rproxy) Read(d []byte) (int, error) {
	return p.brp.Read(d)
}

func (p *rproxy) Close() error {
	p.brp.Reset(nil)
	p.rp.Close()
	p.cg.Done()
	return nil
}

// Split a ReadCloser into 'n' ReadClosers. When all returned ReadClosers have
// been Close()'d, then the underlying ReadCloser is also closed.
func SplitReadCloser(rc io.ReadCloser, n uint) []io.ReadCloser {
	var r []io.ReadCloser
	var w []io.WriteCloser
	cg := new(sync.WaitGroup)
	cg.Add(int(n))
	for i := 0; i < int(n); i++ {
		rp, wp := io.Pipe()
		brp := bufio.NewReader(rp)
		w = append(w, wp)
		rpx := &rproxy{
			i,
			rp,
			brp,
			cg,
		}
		r = append(r, rpx)
	}

	// Closer
	go func() {
		cg.Wait()
		rc.Close()
	}()

	// Read pump
	rpump := func() {
		var err error
		defer func() {
			if err != nil && err != io.EOF {
				log.Errorf("govtil/io/multiplex: rpump error: %v", err)
			}
			for i := 0; i < len(w); i++ {
				w[i].Close()
			}
		}()

		dec := gob.NewDecoder(rc)
		var chno uint
		var d []byte
		for {
			if err = dec.Decode(&chno); err != nil {
				return
			}
			if err = dec.Decode(&d); err != nil {
				return
			}
			if _, err = w[chno].Write(d); err != nil {
				if err == io.ErrClosedPipe {
					// closed sub-channel, keep serving other sub-channels
					log.Debugf("govtil/io/multiplex: sub-channel closed, dumping payload of len %v", len(d))
					continue
				} else {
					// something wrong, stop
					log.Debugf("govtil/io/multiplex: failed to write received payload of length %v to pipe for channel %v", len(d), chno)
					return
				}
			}
		}
	}

	go rpump()
	return r
}
