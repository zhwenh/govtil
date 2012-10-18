// Package mem provides tools for handling and querying memory usage of a
// process
package mem

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"runtime"

	"github.com/vsekhar/govtil/net/server/varz"
)

func MemStats() *runtime.MemStats {
	memstats := new(runtime.MemStats)
	runtime.ReadMemStats(memstats)
	return memstats
}

// A varz function for govtil/net/server/varz
func Varz(w io.Writer) error {
	m := MemStats()
	varz.Write("Alloc", fmt.Sprint(m.Alloc), w)
	varz.Write("TotalAlloc", fmt.Sprint(m.TotalAlloc), w)
	varz.Write("Sys", fmt.Sprint(m.Sys), w)

	varz.Write("HeapAlloc", fmt.Sprint(m.HeapAlloc), w)
	varz.Write("HeapSys", fmt.Sprint(m.HeapSys), w)
	varz.Write("HeapIdle", fmt.Sprint(m.HeapIdle), w)
	varz.Write("HeapInuse", fmt.Sprint(m.HeapInuse), w)
	varz.Write("HeapReleased", fmt.Sprint(m.HeapReleased), w)

	varz.Write("NextGC", fmt.Sprint(m.NextGC), w)
	varz.Write("LastGC", fmt.Sprint(m.LastGC), w)
	varz.Write("PauseTotalNs", fmt.Sprint(m.PauseTotalNs), w)
	for i, v := range m.PauseNs {
		varz.Write("PauseNs["+fmt.Sprint(i)+"]", fmt.Sprint(v), w)
		if i >= 16 {
			break
		}
	}
	varz.Write("NumGC", fmt.Sprint(m.NumGC), w)
	varz.Write("EnableGC", fmt.Sprint(m.EnableGC), w)
	varz.Write("DebugGC", fmt.Sprint(m.DebugGC), w)
	return nil
}

func GC(http.ResponseWriter, *http.Request) {
	runtime.GC()
}

var d []byte

func Create(w http.ResponseWriter, _ *http.Request) {
	var n int64 = 500000000
	d = make([]byte, n)

	// fill it with something to make sure the allocation can't be elided
	for i := 0; i < 10000000; i++ {
		d[rand.Int63n(n)] = 4
	}
	d[len(d)-1] = 7 // write to last element
}

func Delete(http.ResponseWriter, *http.Request) {
	d = nil
}
