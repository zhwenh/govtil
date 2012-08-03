package main

import (
	"govtil/dlog"
	"flag"
	"testing"
	"govtil"
)

func testMain() {
	var tests = []testing.InternalTest{
		{"TestConnection", govtil.TestConnection},
		{"TestSplitSender", govtil.TestSplitSender},
		{"TestSplitReceiver", govtil.TestSplitReceiver},
		{"TestNMuxes", govtil.TestNMuxes},
		{"TestRPC", govtil.TestRPC},
		{"TestXRPC", govtil.TestXRPC},
	}

	flag.Set("test.test", "Test*")
	dlog.SetDebug(true)
	testing.Main(func(string, string) (bool, error) { return true, nil },
		tests,
		[]testing.InternalBenchmark{},
		[]testing.InternalExample{} )
}

func main() {
	testMain()
}
