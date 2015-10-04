package gob

// This file is part of the implementation of govtil/encoding/record, but
// resides in this internal fork of encoding/gob in order to access and/or
// export non-exported members of encoding/gob.

import (
	"reflect"
)


// TODO: move encoder caches into Encoder object, route all references there.
// Add Register function to encoder. Tests might be ok since they register on
// write. What about locks?
//
// Each record.Encoder manages a gob.Encoder (so modified), loads one type only,
// and serializes away.
//
// See clearGobGlobals() for list of caches
//
// No change to decoder? Learns typeids, so they can be anything. We have to
// guarantee typeids are canonicalized at encode time.

// Alternative considered: extract and re-order types from global state, but
// this requires re-writing the typeIds already encoded in all streams. Would
// need to parse out streams or intercept encode, which requires re-writing most
// of encoding/gob.

// clearGobGlobals resets the global caches of the package encoding/gob to their
// initial (program startup) state.
//
// Because the caches are global, all encoders in a program use the same typeId
// space, making it impossible to get "canonical" typeIds for a given type
// without resetting these global caches between invocations.
//
// This problem doesn't exist on the decode side since each decoder learns the
// typeIds specific to its stream during decode.
func clearGobGlobals() {
	// inspired by go1.5.1/encoding/gob/type.go:validUserType()
	userTypeLock.Lock()
	userTypeCache = make(map[reflect.Type]*userTypeInfo)
	userTypeLock.Unlock()

	// inspired by go1.5.1/encoding/gob/type.go:init() and environs
	types = make(map[reflect.Type]gobType)
	idToType = make(map[typeId]gobType)

	// inspired by go1.5.1/encoding/gob/type.go:buildTypeInfo()
	m := make(map[reflect.Type]*typeInfo)
	typeInfoMap.Store(m)

	// inspired by go1.5.1/encoding/gob/type.go:RegisterName()
	registerLock.Lock()
	nameToConcreteType = make(map[string]reflect.Type)
	concreteTypeToName = make(map[reflect.Type]string)
	registerLock.Unlock()

	// Put back built-ins
	nextId = 0
	registerBasics()

	// Primordial types, needed during initialization.
	// Always passed as pointers so the interface{} type
	// goes through without losing its interfaceness.
	bootstrapType("bool", (*bool)(nil), 1)
	bootstrapType("int", (*int)(nil), 2)
	bootstrapType("uint", (*uint)(nil), 3)
	bootstrapType("float", (*float64)(nil), 4)
	bootstrapType("bytes", (*[]byte)(nil), 5)
	bootstrapType("string", (*string)(nil), 6)
	bootstrapType("complex", (*complex128)(nil), 7)
	bootstrapType("interface", (*interface{})(nil), 8)
	// Reserve some Ids for compatible expansion
	bootstrapType("_reserved1", (*struct{ r7 int })(nil), 9)
	bootstrapType("_reserved1", (*struct{ r6 int })(nil), 10)
	bootstrapType("_reserved1", (*struct{ r5 int })(nil), 11)
	bootstrapType("_reserved1", (*struct{ r4 int })(nil), 12)
	bootstrapType("_reserved1", (*struct{ r3 int })(nil), 13)
	bootstrapType("_reserved1", (*struct{ r2 int })(nil), 14)
	bootstrapType("_reserved1", (*struct{ r1 int })(nil), 15)

	checkId(16, mustGetTypeInfo(reflect.TypeOf(wireType{})).id)
	checkId(17, mustGetTypeInfo(reflect.TypeOf(arrayType{})).id)
	checkId(18, mustGetTypeInfo(reflect.TypeOf(CommonType{})).id)
	checkId(19, mustGetTypeInfo(reflect.TypeOf(sliceType{})).id)
	checkId(20, mustGetTypeInfo(reflect.TypeOf(structType{})).id)
	checkId(21, mustGetTypeInfo(reflect.TypeOf(fieldType{})).id)
	checkId(23, mustGetTypeInfo(reflect.TypeOf(mapType{})).id)

	nextId = firstUserId
	userType(reflect.TypeOf((*wireType)(nil)))
}
