package sandbox

import (
	"bytes"
	"fmt"
	"path/filepath"
	"os"
	"os/exec"
	"testing"
)

// invoke sandbox go command on test module
func TestSandbox(t *testing.T) {
	// go test always sets the working directory to the directory containing
	// the package being tested.
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	goroot := filepath.Join(pwd, "go")
	gocc := filepath.Join(goroot, "bin/go")
	testpkg := "./test"

	cmd := exec.Command(gocc, "test", testpkg, "-test.v")
	cmd.Env = []string{
		"GOROOT="+goroot,
		"GOPATH=",
	}
	b := bytes.NewBuffer(nil)
	cmd.Stdout = b
	cmd.Stderr = b
	err = cmd.Run()
	if err != nil {
		fmt.Print(b.String())
		t.Error(err)
	}
}
