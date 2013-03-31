package jail

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"strconv"
	"syscall"
	
	"github.com/vsekhar/govtil/log"
)

// faked starts the faked-sysv daemon, accepting a loadfile and savefile (as
// string paths) for loading and storing the fakeroot environment. The same file
// path can be used for both. A kill channel, a fake daemon channel key (to be
// passed as FAKEROOTKEY env var), and an error are returned.
//
// When the daemon is no longer needed, send any value along the killch to
// tidy up. Consider doing this in a defer function. Failing to signal on the
// killch will leave the daemon running on the system after program termination.
func faked(loadfile, savefile string) (killch chan bool, key int, err error) {
	cmd := exec.Command("faked-sysv")
	if savefile != "" {
		cmd.Args = append(cmd.Args, "--save-file", savefile)
	}
	if loadfile != "" {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, 0, err
		}
		lf, err := os.Open(loadfile)
		if err != nil {
			return nil, 0, err
		}
		defer lf.Close()
		go func() {
			io.Copy(stdin, lf)
			stdin.Close()
		}()
	}
	statusline, err := cmd.Output()
	if err != nil {
		return nil, 0, err
	}
	statuslinestr := strings.TrimSpace(string(statusline))
	parts := strings.Split(statuslinestr, ":")
	if len(parts) != 2 {
		return nil, 0, fmt.Errorf("bad faked statusline: %s", statuslinestr)
	}
	key, err = strconv.Atoi(parts[0])
	if err != nil {
		return nil, 0, err
	}
	pid, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, 0, err
	}

	log.Debugf("faked running at pid %d", pid)
	ch := make(chan bool, 1)
	go func() {
		<-ch // kill signal
		log.Debugf("terminating faked at pid %d", pid)
		err = syscall.Kill(pid, syscall.SIGINT)
		if err != nil {
			log.Errorf("failed to terminate faked at pid %d: %s", pid, err.Error())
		}
		ch <- true // confirmation reply
	}()
	return ch, key, nil
}
