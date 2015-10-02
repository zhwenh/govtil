package multiplex

import (
	"errors"
	"io"
	"net"
	"time"

	viomux "github.com/vsekhar/govtil/io/multiplex"
)

// muxConn implements net.Conn
type muxConn struct {
	io.ReadWriteCloser
	laddr net.Addr
	raddr net.Addr
}

func (mx *muxConn) LocalAddr() net.Addr {
	return mx.laddr
}

func (mx *muxConn) RemoteAddr() net.Addr {
	return mx.raddr
}

func (*muxConn) SetDeadline(time.Time) error {
	return errors.New("muxConn does not implement deadlines")
}

func (*muxConn) SetReadDeadline(time.Time) error {
	return errors.New("muxConn does not implement deadlines")
}

func (*muxConn) SetWriteDeadline(time.Time) error {
	return errors.New("muxConn does not implement deadlines")
}

func Split(c net.Conn, n uint) []net.Conn {
	rwcs := viomux.SplitReadWriteCloser(c, n)
	var r []net.Conn
	laddr := c.LocalAddr()
	raddr := c.RemoteAddr()
	for _, rwc := range rwcs {
		r = append(r, &muxConn{rwc, laddr, raddr})
	}
	return r
}
