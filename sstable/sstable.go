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

var NotFound error = errors.New("key not found")

// Flush writes an ssTable and its index to an io.WriteSeeker. It returns the
// index into the io.WriteSeeker and an error.
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

	// write keys and values in sorted order
	for _, k := range keys {
		// key
		if err := enc.Encode(k); err != nil {
			return idx, err
		}

		// length of value (for skipping values during scanning)
		if err := enc.Encode(len(s[k])); err != nil {
			return idx, err
		}

		// value
		loc, err := w.Seek(0, os.SEEK_CUR) // for index
		if err != nil {
			return idx, err
		}
		if err := enc.Encode(s[k]); err != nil {
			return idx, err
		}
		idx[k] = loc
	}

	// write index as follows:
	//  idx: n bytes (gob)
	//  idx_start: k bytes (absolute position, gob)
	//  k: 1 byte (offset from end of file, raw byte)
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

// for testing
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

type SSTableReader struct {
	f io.ReadSeeker
	idx map[string]int64
}

func (s *SSTableReader) Get(k string) ([]byte, error) {
	cur_pos, err := s.f.Seek(0, os.SEEK_CUR)
	if err != nil {
		return nil, err
	}
	loc, ok := s.idx[k]
	if !ok {
		return nil, NotFound
	}
	_, err = s.f.Seek(loc, os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	b := []byte{}
	dec := gob.NewDecoder(s.f)
	if err = dec.Decode(&b); err != nil {
		return b, err
	}
	_, err = s.f.Seek(cur_pos, os.SEEK_SET)
	if err != nil {
		return b, err
	}
	return b, nil
}

func (s *SSTableReader) Len() int64 {
	return len(s.idx)
}

func LoadIndex(r io.ReadSeeker) (SSTableReader, error) {
	cur_pos, err := r.Seek(0, os.SEEK_CUR) // stash position
	if err != nil {
		return SSTableReader{}, err
	}

	// read offset from file_end
	r.Seek(-1, os.SEEK_END)
	b := []byte{0}
	_, err = r.Read(b)
	if err != nil {
		return SSTableReader{}, err
	}
	offset := int64(b[0]) + 1

	// read idx_start
	r.Seek(-offset, os.SEEK_END)
	dec := gob.NewDecoder(r)
	var idx_start int64
	if err := dec.Decode(&idx_start); err != nil {
		return SSTableReader{}, err
	}
	
	// read idx
	r.Seek(idx_start, os.SEEK_SET)
	dec = gob.NewDecoder(r) // flush buffer
	var idx map[string]int64
	if err := dec.Decode(&idx); err != nil {
		return SSTableReader{}, err
	}
	
	// restore position
	_, err = r.Seek(cur_pos, os.SEEK_SET)
	if err != nil {
		return SSTableReader{}, err
	}

	return SSTableReader{f: r, idx: idx}, nil
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
