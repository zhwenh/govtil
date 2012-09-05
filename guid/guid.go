package guid

import (
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"net"
	"time"
	
	vbytes "github.com/vsekhar/govtil/bytes"
)

const GUIDLength = 16
type sha1GUID [GUIDLength]byte
var baseGUID sha1GUID

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

type GUID interface {
	String() string
	Short() string
	Equals(GUID) bool
	bytes() [GUIDLength]byte
}

func (sg *sha1GUID) String() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", sg[0:4], sg[4:6], sg[6:8], sg[8:10], sg[10:])
}

func (sg *sha1GUID) Short() string {
	return fmt.Sprintf("%x", sg[12:])
}

func (sg *sha1GUID) Equals(sg2 GUID) bool {
	b := sg2.bytes()
	return vbytes.Equals(sg[:], b[:])
}

func (sg *sha1GUID) bytes() [GUIDLength]byte {
	return *sg
}

func setV5(b []byte) {
	// Fix bytes to resemble xxxxxxxx-xxxx-5xxx-Yxxx-xxxxxxxxxxxx where x is
	// any hex value and Y is one of 8, 9, A or B
	// 8 = 0
	b[6] = 0x50 + (b[6] & 0x0f)
	b[8] = 0x80 + (b[8] & 0x3f)
}

func New() (GUID, error) {
	ret := new(sha1GUID)
	_, err := rand.Read(ret[:])
	if err != nil {
		return ret, err
	}
	setV5(ret[:])
	return ret, nil
}
