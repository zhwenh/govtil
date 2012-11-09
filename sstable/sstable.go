package sstable

import (
	"encoding/gob"
	"errors"
	"io"
	"os"
	"sort"
)

// TODO(vsekhar): ssTable struct with value-getter, cache, own and manage
// backing file, etc.

type ssTable map[string][]byte

func Flush(s ssTable, w io.WriteSeeker) (map[string]int64, error) {
	enc := gob.NewEncoder(w)
	keys := make([]string, len(s))
	idx := make(map[string]int64)
	i := 0
	for k := range s {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := enc.Encode(k); err != nil {
			return idx, err
		}
		loc, err := w.Seek(0, os.SEEK_CUR)
		if err != nil {
			return idx, err
		}
		idx[k] = loc
		if err := enc.Encode(s[k]); err != nil {
			return idx, err
		}
	}
	// write index as follows:
	//  idx: n bytes (gob)
	//  idx_start: k bytes (abs pos, gob)
	//  k: 1 byte (raw)

	// record idx_start, write idx, record idx_end
	idx_start, err := w.Seek(0, os.SEEK_CUR)
	if err != nil {
		return idx, err
	}
	if err := enc.Encode(idx); err != nil {
		return idx, err
	}
	idx_end, err := w.Seek(0, os.SEEK_CUR)
	if err != nil {
		return idx, err
	}

	// write idx_start, record file_end
	if err := enc.Encode(idx_start); err != nil {
		return idx, err
	}
	file_end, err := w.Seek(0, os.SEEK_CUR)
	if err != nil {
		return idx, err
	}

	// write offset from file_end
	if file_end - idx_end > 255 {
		return idx, errors.New("gob-encoded int64 has length > 255")
	}
	offset := byte(file_end - idx_end) // won't overflow
	_, err = w.Write([]byte{offset})
	if err != nil {
		return idx, err
	}
	
	return idx, nil
}

func getKV(dec *gob.Decoder) (string, []byte, error) {
	k := ""
	v := []byte{}
	if err := dec.Decode(&k); err != nil {
		return "", nil, err
	}
	if err := dec.Decode(&v); err != nil {
		return "", nil, err
	}
	return k, v, nil
}

func LoadIndex(r io.ReadSeeker) (map[string]int64, error) {
	cur_pos, err := r.Seek(0, os.SEEK_CUR) // stash position
	if err != nil {
		return nil, err
	}

	// read offset from file_end
	r.Seek(-1, os.SEEK_END)
	b := []byte{0}
	_, err = r.Read(b)
	if err != nil {
		return nil, err
	}
	offset := int64(b[0]) + 1

	// read idx_start
	r.Seek(-offset, os.SEEK_END)
	dec := gob.NewDecoder(r)
	var idx_start int64
	if err := dec.Decode(&idx_start); err != nil {
		return nil, err
	}
	
	// read idx
	r.Seek(idx_start, os.SEEK_SET)
	dec = gob.NewDecoder(r) // flush buffer
	var idx map[string]int64
	if err := dec.Decode(&idx); err != nil {
		return nil, err
	}
	
	// restore position
	_, err = r.Seek(cur_pos, os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	
	return idx, nil
}

// TODO(vsekhar): remove this, it will be infeasible, use cache and getters
// instead.
func Load(r io.ReadSeeker) (ssTable, error) {
	idx, err := LoadIndex(r)
	if err != nil {
		return nil, err
	}
	n := len(idx)
	r.Seek(0, os.SEEK_SET)
	dec := gob.NewDecoder(r)
	s := ssTable{}
	for i := 0; i < n; i++ {
		k, v, err := getKV(dec)
		if err != nil {
			if err == io.EOF {
				break
			}
			return s, err
		}
		s[k] = v
	}
	return s, nil
}

func getLoc(r io.ReadSeeker, loc int64) ([]byte, error) {
	dec := gob.NewDecoder(r)
	_, err := r.Seek(loc, os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	v := []byte{}
	if err = dec.Decode(&v); err != nil {
		return v, err
	}
	return v, nil
}
