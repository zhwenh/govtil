package gob

// This file is part of the implementation of govtil/encoding/record, but
// resides in this internal fork of encoding/gob in order to access and/or
// export non-exported members of encoding/gob.

import (
	"fmt"
	"reflect"
	"testing"
)

// Initial numbers of elements in gob global caches
const (
	userTypeCacheLen = 58
	typesLen = 25
	idToTypeLen = 24
	typeInfoMapLen = 7
	nameToConcreteTypeLen = 34
	concreteTypeToNameLen = 34
)

// testCacheSizes checks each cache to ensure it has the known initial number of
// elements.
func testInitialCacheSizes() error {
	userTypeLock.RLock()
	if len(userTypeCache) != userTypeCacheLen {
		return fmt.Errorf("userTypeCache has %d, should have %d", len(userTypeCache), userTypeCacheLen)
	}
	userTypeLock.RUnlock()
	if len(types) != typesLen {
		return fmt.Errorf("types has %d, should have %d", len(types), typesLen)
	}
	if len(idToType) != idToTypeLen {
		return fmt.Errorf("idToTypeLen has %d, should have %d", len(idToType), idToTypeLen)
	}
	l := len(typeInfoMap.Load().(map[reflect.Type]*typeInfo))
	if l != typeInfoMapLen {
		return fmt.Errorf("typeInfoMap has %d, should have %d", typeInfoMapLen, l)
	}
	registerLock.Lock()
	if len(nameToConcreteType) != nameToConcreteTypeLen {
		return fmt.Errorf("nameToConcreteType has %d, should have %d", len(nameToConcreteType), nameToConcreteTypeLen)
	}
	if len(concreteTypeToName) != concreteTypeToNameLen {
		return fmt.Errorf("concreteTypeToName has %d, should have %d", len(concreteTypeToName), concreteTypeToNameLen)
	}
	registerLock.Unlock()
	if nextId != firstUserId {
		return fmt.Errorf("nextId is %d, should be %d", nextId, firstUserId)
	}
	return nil
}

var initErr error

func init() {
	initErr = testInitialCacheSizes() // run before other gob tests
}

func TestRecordInitialCacheSizes(t *testing.T) {
	if initErr != nil {
		t.Error(initErr)
	}
}

func TestRecordClearGobGlobalsInitial(t *testing.T) {
	// Ensure clearGobGlobals resets to known initial state
	clearGobGlobals()
	if err := testInitialCacheSizes(); err != nil {
		t.Error(err)
	}
}

type t1 struct {
	A int
}

type t2 struct {
	B byte
	T t1
}

type t3 struct {
	AnotherField float32
}

func TestRecordClearGobGlobals(t *testing.T) {
	Register(t3{})
	checkId(65, mustGetTypeInfo(reflect.TypeOf(t3{})).id)
	clearGobGlobals()
	if err := testInitialCacheSizes(); err != nil {
		t.Error(err)
	}
}

func TestRecordCacheContents(t *testing.T) {
	// reset caches
	// Register() a single user type
	// copy all cache values
	// reset caches
	// Register
}
