package sandbox

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/vsekhar/govtil/log"
)

var sandboxDir string
var sandboxGoRoot string
var sandboxGoCc string

func cmd(c string, a ...string) (*exec.Cmd, *bytes.Buffer) {
	cmd := exec.Command(c, a...)
	b := bytes.NewBuffer(nil)
	cmd.Stdout = b
	cmd.Stderr = b
	return cmd, b
}

func run(c string, a ...string) (string, error) {
	cmd, b := cmd(c, a...)
	err := cmd.Run()
	if err != nil {
		return b.String(), err
	}
	return "", nil
}

func copyLocal() error {
	log.Always("trying to copy local")
	goRoot := os.Getenv("GOROOT")
	if goRoot == "" {
		log.Always("no installed go")
		return errors.New("no installed Go")
	}

	if _, err := os.Stat(sandboxGoRoot); os.IsNotExist(err) {
		if err = os.MkdirAll(sandboxGoRoot, 0644); err != nil {
			return err
		}
	}
	if s, err := run("cp", "-r", filepath.Join(goRoot, "bin"), sandboxGoRoot); err != nil {
		log.Always(s)
		return err
	}
	if s, err := run("cp", "-r", filepath.Join(goRoot, "pkg"), sandboxGoRoot); err != nil {
		log.Always(s)
		return err
	}
	if s, err := run("cp", "-r", filepath.Join(goRoot, "src"), sandboxGoRoot); err != nil {
		log.Always(s)
		return err
	}
	if s, err := run("cp", "-r", filepath.Join(goRoot, "VERSION"), sandboxGoRoot); err != nil {
		log.Always(s)
		return err
	}
	return nil
}

func getRemote() error {
	log.Always("trying to get remote Go source")
	out, err := run("hg", "clone", "-r release", "-b default", "-u release", "https://code.google.com/p/go")
	if err != nil {
		log.Error(out)
		return err
	}

	log.Always("building")
	cmd, b := cmd("./all.bash")
	cmd.Dir = filepath.Join(sandboxGoRoot, "src")
	err = cmd.Run()
	if err != nil {
		log.Error(b.String())
		return err
	}
	return nil
}

func clearDir(exceptions []string) error {
	e := make(map[string]bool)
	for _, s := range exceptions {
		e[s] = true
	}
	dir, err := os.Open(filepath.Join(sandboxGoRoot, "bin"))
	if err != nil {
		return err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, n := range names {
		if _, ok := e[n]; !ok {
			os.Remove(n)
		}
	}
	return nil
}

func clean() error {
	return nil
}

func init() {
	var err error
	sandboxDir, err = os.Getwd()
	if err != nil {
		panic(err)
	}
	sandboxGoRoot = filepath.Join(sandboxDir, "go")
	sandboxGoCc = filepath.Join(sandboxGoRoot, "bin/go")

	// check for sandbox compiler
	_, err = os.Stat(sandboxGoCc)
	haveSandboxCompiler := !os.IsNotExist(err)

	// TODO: check for compiled runtime in goRoot/pkg/*/runtime.a

	if !haveSandboxCompiler /* || !haveSandboxRuntime */ {
		log.Always("Sandboxed Go not found")
		// try local, then remote
		if err := copyLocal(); err != nil {
			if err := getRemote(); err != nil {
				panic(err)
			}
		}
	}
}

// invoke sandbox go command on test module
func TestSandbox(t *testing.T) {
	// go test always sets the working directory to the directory containing
	// the package being tested.
	testpkg := "./test/sandbox_test.go"

	cmd := exec.Command(sandboxGoCc, "test", testpkg, "-test.v")
	cmd.Env = []string{
		"GOROOT=" + sandboxGoRoot,
		"GOPATH=", // TODO: add path to other libraries available to the sandbox
	}
	b := bytes.NewBuffer(nil)
	cmd.Stdout = b
	cmd.Stderr = b
	err := cmd.Run()
	if err != nil {
		fmt.Print(b.String())
		t.Error(err)
	}
}
