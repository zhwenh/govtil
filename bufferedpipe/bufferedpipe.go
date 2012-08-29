package bufferedpipe

import (
	"io"
)

const defaultBufSize = 4096

func New() (*io.PipeReader, *io.PipeWriter) {
	p1reader, p1writer := io.Pipe()
	p2reader, p2writer := io.Pipe()
	
	go func(reader *io.PipeReader, writer *io.PipeWriter) {
		var r_err error = nil
		var w_err error = nil
		defer func() {
			if r_err != nil {
				writer.CloseWithError(r_err)
			} else {
				writer.Close()
			}
			if w_err != nil {
				reader.CloseWithError(w_err)
			} else {
				reader.Close()
			}
		}()

		buffer := make([]byte, defaultBufSize)
		for {
			n, r_err := reader.Read(buffer)
			if r_err != nil {
				return
			}
			_, w_err := writer.Write(buffer[:n])
			if w_err != nil {
				return
			}
		}
	}(p2reader, p1writer)
	
	return p1reader, p2writer
}
