/*
	Package log provides a logger with variable verbosity.
*/
package log

import (
	"flag"
	stdlog "log"
	"sync"
)

var verbosity = flag.Int("verbosity", 0, "logging verbosity (-1==QUIET to 1==DEBUG)")

const (
	QUIET = -1
	NORMAL = 0
	DEBUG = 1
)

type Level int
var mutex sync.RWMutex

func test(level Level) bool {
	mutex.RLock()
	defer mutex.RUnlock()
	return Level(*verbosity) >= level
}

func SetVerbosity(level Level) {
	mutex.Lock()
	defer mutex.Unlock()
	*verbosity = int(level)
}

func Log(level Level, args ...interface{}) { if test(level) { stdlog.Print(args...) } }
func Logf(level Level, s string, args ...interface{}) { if test(level) { stdlog.Printf(s, args...) } }
func Logln(level Level, args ...interface{}) { if test(level) { stdlog.Println(args...) } }

func Debug(args ...interface{}) { if test(DEBUG) { stdlog.Print(args...) } }
func Debugf(s string, args ...interface{}) { if test(DEBUG) { stdlog.Printf(s, args...) } }
func Debugln(args ...interface{}) { if test(DEBUG) { stdlog.Println(args...) } }

func Print(args ...interface{}) { if test(NORMAL) { stdlog.Print(args...) } }
func Printf(s string, args ...interface{}) { if test(NORMAL) { stdlog.Printf(s, args...) } }
func Println(args ...interface{}) { if test(NORMAL) { stdlog.Println(args...) } }

// Always log fatals
var Fatal = stdlog.Fatal
var Fatalf = stdlog.Fatalf
var Fatalln = stdlog.Fatalln
