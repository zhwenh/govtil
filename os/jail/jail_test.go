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

func connectedCommand(j Interface, c string) *exec.Cmd {
	cmd := Command(c)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd
}

const lxcScript = `
ifconfig
ping -c 1 google.com
`

func TestLxcJail(t *testing.T) {
	l, err := NewLxcJail("/home/vsekhar/chroot", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := l.CleanUp(); err != nil {
			t.Error(err)
		}
	}()

	cmds := strings.Split(lxcScript, "\n")
	for _, cmd := range cmds {
		c := Command(cmd)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Stdin = os.Stdin

		t.Logf("Running %s", c.Path)
		if err := l.Run(c); err != nil {
			t.Fatal(err)
		}
	}
}
