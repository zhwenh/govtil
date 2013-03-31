package jail

import (
	"os"
	"os/signal"
	"testing"
)

func TestFaked(t *testing.T) {
	killch, _, err := faked("", "")
	if err != nil {
		t.Error(err)
	} else {
		killch <- true
		<-killch // confirm done
	}
}

// TODO(vsekhar): test NewChrootJail()
// TODO(vsekhar): test NewLxcJail()

func TestLxcJail(t *testing.T) {
	lxc, err := NewLxcJail("/home/vsekhar/chroot", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lxc.CleanUp(); err != nil {
			t.Error(err)
		}
	}()

	// cmd := exec.Command("ifconfig")
	// cmd := exec.Command("bash", "-c", "echo hello; echo goodbye")
	// cmd := exec.Command("bash", "-c", "env")
	// cmd := exec.Command("ping", "-c", "1", "74.125.224.105")
	cmd := Command("ping google.com")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if cmd, err = lxc.Imprison(cmd); err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Forward Ctrl-C to underlying process, to allow Go to cleanup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for s := range c {
			cmd.Process.Signal(s)
		}
	}()
	if err := cmd.Wait(); err != nil {
		t.Error(err)
	}

}
