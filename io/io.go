package io

import (
	stdio "io"
)

type rwc struct {
	stdio.ReadCloser
	stdio.WriteCloser
}

func (this *rwc) Close() error {
	ec := make(chan error)
	go func() {
		err := this.ReadCloser.Close();
		ec <- err
	}()

	if err := this.WriteCloser.Close(); err != nil {
		return err
	}
	if err := <-ec; err != nil {
		return err
	}
	return nil
}

// NewReadWriteCloser combines an io.ReadCloser with an io.WriteCloser. When the
// returned io.ReadWriteCloser is closed, both the ReadCloser and WriteCloser
// are closed in parallel.
func NewReadWriteCloser(r stdio.ReadCloser, w stdio.WriteCloser) stdio.ReadWriteCloser {
	return &rwc{r, w}
}
