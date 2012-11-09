package sstable

import (
	"encoding/gob"
	"io/ioutil"
	"os"
	"sort"
	"testing"
)

func makeSSTable() ssTable {
	s := ssTable{
		"hello": []byte("blah"),
		"hello2": []byte("anotherblah"),
		"abc123": []byte("zingo"),
	}
	return s
}

func flushSSTable(s ssTable) (*os.File, map[string]int64, error) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, nil, err
	}
	idx, err := Flush(s, f)
	if err != nil {
		return f, idx, err
	}
	return f, idx, nil
}

func TestSSTable(t *testing.T) {
	s := makeSSTable()
	f, _, err := flushSSTable(s)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.Seek(0, os.SEEK_SET)
	s2, err := Load(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != len(s2) {
		t.Error("lengths different after Flush/Load")
	}
	for k,v := range s {
		if s2[k] == nil {
			t.Errorf("did not find key %s after Flush/Load", k) 
		}
		if string(v) != string(s2[k]) {
			t.Errorf("values don't match: %s and %s", v, s2[k])
		}
	}
}

func TestSort(t *testing.T) {
	s := makeSSTable()
	f, _, err := flushSSTable(s)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.Seek(0, os.SEEK_SET)

	// Read keys and ensure sorted on disk
	idx, err := LoadIndex(f)
	if err != nil {
		t.Fatal(err)
	}
	n := len(idx)
	f.Seek(0, os.SEEK_SET)
	dec := gob.NewDecoder(f)
	pk := ""
	first := true
	for i := 0; i < n; i++ {
		k, _, err := getKV(dec)
		if err != nil {
			t.Fatal(err)
		}
		if !first && k < pk {
			t.Errorf("bad key order: %s then %s", pk, k)
		}
		pk = k
		first = false
	}
}

func TestIndexGet(t *testing.T) {
	s := makeSSTable()
	f, idx, err := flushSSTable(s)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Seek(0, os.SEEK_SET)
	
	keys := make([]string, len(idx))
	i := 0
	for k := range idx {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	first := true
	pk := ""
	ploc := int64(0)
	for _, k := range keys {
		if !first && k < pk {
			t.Errorf("bad key order: %s then %s", pk, k)
		}
		if idx[k] <= ploc {
			t.Errorf("bad index loc order: %d then %d", ploc, idx[k])
		}
		first = false
		pk = k
		ploc = idx[k]
	}
}
