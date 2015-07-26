package signal

import (
	"os"
	stdsignal "os/signal"
	"syscall"
)

// Do runs f when any operating system stop signal (such as SIGINT, SIGKILL, and
// some others) are received by the process.
//
// This can be used, for example, to gracefully shutdown a web server
// (http.Serve() will return when its listen socket is closed).
func Go(f func(os.Signal)) {
	stopsigs := []os.Signal{
		syscall.SIGABRT,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGPWR,
		syscall.SIGQUIT,
		syscall.SIGSTOP,
		syscall.SIGTERM,
	}
	GoCustom(f, stopsigs)
}

func GoCustom(f func(os.Signal), sigs []os.Signal) {
	sigch := make(chan os.Signal)
	stdsignal.Notify(sigch, sigs...)
	go func() {
		f(<-sigch)
	}()
}
