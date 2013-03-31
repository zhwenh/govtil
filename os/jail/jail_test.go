package jail

import (
	"os"
	"os/exec"
	"strings"
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
	lxc, err := NewLxcJail("/home/vsekhar/chroot")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lxc.CleanUp(); err != nil {
			t.Error(err)
		}
	}()

	// cmd := exec.Command("ifconfig")
	cmd := exec.Command("ping", "74.125.224.136") // google.com
	// cmd := exec.Command("pwd")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if cmd, err = lxc.Imprison(cmd); err != nil {
		t.Fatal(err)
	}
	t.Logf("Commandline: %s", strings.Join(cmd.Args, " "))
	if err = cmd.Run(); err != nil {
		t.Error(err)
	}
}
