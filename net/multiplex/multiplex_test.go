package multiplex

import (
	"io"
	"testing"

	"github.com/vsekhar/govtil/io/multiplex"
	vtesting "github.com/vsekhar/govtil/testing"
)

func TestSplitConn(t *testing.T) {
	c1, c2 := vtesting.SelfConnection()
	c1s := Split(c1, 2)
	c2s := Split(c2, 2)
	var rwcs1 []io.ReadWriteCloser
	var rwcs2 []io.ReadWriteCloser
	for _, c := range c1s {
		rwcs1 = append(rwcs1, c)
	}
	for _, c := range c2s {
		rwcs2 = append(rwcs2, c)
	}

	multiplex.DoTestBiDirectional(t, rwcs1, rwcs2)
}
