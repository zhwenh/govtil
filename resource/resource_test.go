package resource

import (
	"errors"
	"testing"
)

var counter int

func inc() error { counter += 1; return nil }
func dec() error { counter -= 1; return nil }

func TestResource(t *testing.T) {
	counter = 0
	rm := &ResourceManager{}
	n := 10
	for i := 0; i < n; i++ {
		if err := rm.Acquire(inc, dec); err != nil {
			t.Fatal("failed to acquire")
		}
	}
	if counter != n {
		t.Fatalf("failed to acquire, expected %d, result %d", n, counter)
	}
	if err := rm.Release(); err != nil {
		t.Fatalf("failed to release: %s", err)
	}
	if counter != 0 {
		t.Errorf("failed to release, expected 0, result %d", counter)
	}
}

type chainable struct {
	bool
	*chainable
}

func (c *chainable) set() error {
	c.bool = true
	c.chainable = new(chainable)
	return nil
}

func (c *chainable) reset() error {
	if c.chainable != nil && c.chainable.bool {
		return errors.New("out of order release")
	}
	if c.bool != true {
		return errors.New("reset of unset chainable")
	}
	c.bool = false
	return nil
}

func TestResourceOrdered(t *testing.T) {
	rm := &ResourceManager{}
	c := new(chainable)
	n := 10
	for i := 0; i < n; i++ {
		cc := c
		if err := rm.Acquire(func() error { return cc.set() }, func() error { return cc.reset() }); err != nil {
			t.Fatalf("failed to acquire: %s", err)
		}
		c = c.chainable
	}
	if err := rm.Release(); err != nil {
		t.Fatalf("failed to release: %s", err)
	}
}
