package pipes

import (
	"io"
)

const defaultBufSize = 4096

// Buffered returns two ends of a buffered in-memory pipe
func Buffered() (*io.PipeReader, *io.PipeWriter) {
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

type BiPipe struct {
	*io.PipeReader
	*io.PipeWriter
}

func (bi *BiPipe) Close() error {
	err1 := bi.PipeWriter.Close()
	err2 := bi.PipeReader.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (bi *BiPipe) CloseWithError(err error) error {
	err1 := bi.PipeWriter.CloseWithError(err)
	err2 := bi.PipeReader.CloseWithError(err)
	if err1 != nil {
		return err1
	}
	return err2
}

// Bi returns two ends of a bi-directional unbuffered in-memory pipe
func Bi() (BiPipe, BiPipe) {
	p1reader, p1writer := io.Pipe()
	p2reader, p2writer := io.Pipe()
	return BiPipe{p1reader, p2writer}, BiPipe{p2reader, p1writer}
}

// BiBuffered returns two ends of a bi-directional buffered in-memory pipe
func BiBuffered() (BiPipe, BiPipe) {
	p1reader, p1writer := Buffered()
	p2reader, p2writer := Buffered()
	return BiPipe{p1reader, p2writer}, BiPipe{p2reader, p1writer}
}
