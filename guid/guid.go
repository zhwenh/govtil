package guid

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
	
	vbytes "github.com/vsekhar/govtil/bytes"
)

const GUIDLength = 16
type GUID [GUIDLength]byte
var baseGUID GUID

func init() {
	// Create a time- and hardware address-based baseGUID
	ifs, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	if len(ifs) == 0 {
		panic(errors.New("No hardware network interfaces detected"))
	}
	hasher := sha1.New()
	hasher.Write([]byte(time.Now().String()))
	for _,i := range ifs {
		hasher.Write(i.HardwareAddr)
	}
	copy(baseGUID[:], hasher.Sum(nil))
}

// Return the GUID as a string in 8-4-4-4-12 format, e.g.
//	"550e8400-e29b-41d4-a716-665544a6a71e"
func (sg *GUID) String() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", sg[0:4], sg[4:6], sg[6:8], sg[8:10], sg[10:])
}

// Return the GUID as a short string representing only the last 4 bytes, e.g.
//	"44a6a71e"
func (sg *GUID) Short() string {
	return fmt.Sprintf("%x", sg[12:])
}

func (sg *GUID) Equals(sg2 *GUID) bool {
	return vbytes.Equals(sg[:], sg2[:])
}

func (sg *GUID) bytes() [GUIDLength]byte {
	return *sg
}

func V4() (GUID, error) {
	ret := GUID{}
	_, err := rand.Read(ret[:])
	if err != nil {
		return ret, err
	}
	// Mix with something host- and time-dependent
	for i := 0; i < GUIDLength; i++ {
		ret[i] ^= baseGUID[i]
	}

	// Template: xxxxxxxx-xxxx-4xxx-Yxxx-xxxxxxxxxxxx where x is
	// any hex value and Y is one of 8, 9, A or B
	ret[6] = 0x40 + (ret[6] & 0x0f)
	ret[8] = 0x80 + (ret[8] & 0x3f)
	return ret, nil
}

func V5FromReader(r io.Reader) (guid GUID, err error) {
	ret := GUID{}
	hasher := sha1.New()
	_, err = io.Copy(hasher, r)
	if err != nil {
		return
	}
	copy(ret[:], hasher.Sum(nil))

	// Template: xxxxxxxx-xxxx-5xxx-Yxxx-xxxxxxxxxxxx where x is
	// any hex value and Y is one of 8, 9, A or B
	ret[6] = 0x50 + (ret[6] & 0x0f)
	ret[8] = 0x80 + (ret[8] & 0x3f)
	return ret, nil
}

func V5FromBytes(b []byte) (guid GUID, err error) {
	return V5FromReader(bytes.NewBuffer(b))
}
