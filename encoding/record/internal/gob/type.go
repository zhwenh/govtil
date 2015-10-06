// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gob

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

// userTypeInfo stores the information associated with a type the user has handed
// to the package.  It's computed once and stored in a map keyed by reflection
// type.
type userTypeInfo struct {
	user        reflect.Type // the type the user handed us
	base        reflect.Type // the base type after all indirections
	indir       int          // number of indirections to reach the base type
	externalEnc int          // xGob, xBinary, or xText
	externalDec int          // xGob, xBinary or xText
	encIndir    int8         // number of indirections to reach the receiver type; may be negative
	decIndir    int8         // number of indirections to reach the receiver type; may be negative
}

// externalEncoding bits
const (
	xGob    = 1 + iota // GobEncoder or GobDecoder
	xBinary            // encoding.BinaryMarshaler or encoding.BinaryUnmarshaler
	xText              // encoding.TextMarshaler or encoding.TextUnmarshaler
)

var (
	// Protected by an RWMutex because we read it a lot and write
	// it only when we see a new type, typically when compiling.
	userTypeLock  sync.RWMutex
	userTypeCache = make(map[reflect.Type]*userTypeInfo)
)

// validType returns, and saves, the information associated with user-provided type rt.
// If the user type is not valid, err will be non-nil.  To be used when the error handler
// is not set up.
func validUserType(rt reflect.Type) (ut *userTypeInfo, err error) {
	userTypeLock.RLock()
	ut = userTypeCache[rt]
	userTypeLock.RUnlock()
	if ut != nil {
		return
	}
	// Now set the value under the write lock.
	userTypeLock.Lock()
	defer userTypeLock.Unlock()
	if ut = userTypeCache[rt]; ut != nil {
		// Lost the race; not a problem.
		return
	}
	ut = new(userTypeInfo)
	ut.base = rt
	ut.user = rt
	// A type that is just a cycle of pointers (such as type T *T) cannot
	// be represented in gobs, which need some concrete data.  We use a
	// cycle detection algorithm from Knuth, Vol 2, Section 3.1, Ex 6,
	// pp 539-540.  As we step through indirections, run another type at
	// half speed. If they meet up, there's a cycle.
	slowpoke := ut.base // walks half as fast as ut.base
	for {
		pt := ut.base
		if pt.Kind() != reflect.Ptr {
			break
		}
		ut.base = pt.Elem()
		if ut.base == slowpoke { // ut.base lapped slowpoke
			// recursive pointer type.
			return nil, errors.New("can't represent recursive pointer type " + ut.base.String())
		}
		if ut.indir%2 == 0 {
			slowpoke = slowpoke.Elem()
		}
		ut.indir++
	}

	if ok, indir := implementsInterface(ut.user, gobEncoderInterfaceType); ok {
		ut.externalEnc, ut.encIndir = xGob, indir
	} else if ok, indir := implementsInterface(ut.user, binaryMarshalerInterfaceType); ok {
		ut.externalEnc, ut.encIndir = xBinary, indir
	}

	// NOTE(rsc): Would like to allow MarshalText here, but results in incompatibility
	// with older encodings for net.IP. See golang.org/issue/6760.
	// } else if ok, indir := implementsInterface(ut.user, textMarshalerInterfaceType); ok {
	// 	ut.externalEnc, ut.encIndir = xText, indir
	// }

	if ok, indir := implementsInterface(ut.user, gobDecoderInterfaceType); ok {
		ut.externalDec, ut.decIndir = xGob, indir
	} else if ok, indir := implementsInterface(ut.user, binaryUnmarshalerInterfaceType); ok {
		ut.externalDec, ut.decIndir = xBinary, indir
	}

	// See note above.
	// } else if ok, indir := implementsInterface(ut.user, textUnmarshalerInterfaceType); ok {
	// 	ut.externalDec, ut.decIndir = xText, indir
	// }

	userTypeCache[rt] = ut
	return
}

var (
	gobEncoderInterfaceType        = reflect.TypeOf((*GobEncoder)(nil)).Elem()
	gobDecoderInterfaceType        = reflect.TypeOf((*GobDecoder)(nil)).Elem()
	binaryMarshalerInterfaceType   = reflect.TypeOf((*encoding.BinaryMarshaler)(nil)).Elem()
	binaryUnmarshalerInterfaceType = reflect.TypeOf((*encoding.BinaryUnmarshaler)(nil)).Elem()
	textMarshalerInterfaceType     = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	textUnmarshalerInterfaceType   = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

// implementsInterface reports whether the type implements the
// gobEncoder/gobDecoder interface.
// It also returns the number of indirections required to get to the
// implementation.
func implementsInterface(typ, gobEncDecType reflect.Type) (success bool, indir int8) {
	if typ == nil {
		return
	}
	rt := typ
	// The type might be a pointer and we need to keep
	// dereferencing to the base type until we find an implementation.
	for {
		if rt.Implements(gobEncDecType) {
			return true, indir
		}
		if p := rt; p.Kind() == reflect.Ptr {
			indir++
			if indir > 100 { // insane number of indirections
				return false, 0
			}
			rt = p.Elem()
			continue
		}
		break
	}
	// No luck yet, but if this is a base type (non-pointer), the pointer might satisfy.
	if typ.Kind() != reflect.Ptr {
		// Not a pointer, but does the pointer work?
		if reflect.PtrTo(typ).Implements(gobEncDecType) {
			return true, -1
		}
	}
	return false, 0
}

// userType returns, and saves, the information associated with user-provided type rt.
// If the user type is not valid, it calls error.
func userType(rt reflect.Type) *userTypeInfo {
	ut, err := validUserType(rt)
	if err != nil {
		error_(err)
	}
	return ut
}

// A typeId represents a gob Type as an integer that can be passed on the wire.
// Internally, typeIds are used as keys to a map to recover the underlying type info.
type typeId int32

var typeLock sync.Mutex // set while building a type
const firstUserId = 64  // lowest id number granted to user

type gobType interface {
	id() typeId
	setId(id typeId)
	name() string
	string() string // not public; only for debugging
	safeString(seen map[typeId]bool) string
}

var builtinIdToType map[typeId]gobType // set in init() after builtins are established

func (enc *Encoder) setTypeId(typ gobType) {
	// When building recursive types, someone may get there before us.
	if typ.id() != 0 {
		return
	}
	enc.nextId++
	typ.setId(enc.nextId)
	enc.idToType[enc.nextId] = typ
}

func (enc *Encoder) gobType(t typeId) gobType {
	if t == 0 {
		return nil
	}
	return enc.idToType[t]
}

// string returns the string representation of the type associated with the typeId.
func (enc *Encoder) string(t typeId) string {
	if enc.gobType(t) == nil {
		return "<nil>"
	}
	return enc.gobType(t).string()
}

// Name returns the name of the type associated with the typeId.
func (enc *Encoder) name(t typeId) string {
	if enc.gobType(t) == nil {
		return "<nil>"
	}
	return enc.gobType(t).name()
}

// CommonType holds elements of all types.
// It is a historical artifact, kept for binary compatibility and exported
// only for the benefit of the package's encoding of type descriptors. It is
// not intended for direct use by clients.
type CommonType struct {
	Name string
	Id   typeId
}

func (t *CommonType) id() typeId { return t.Id }

func (t *CommonType) setId(id typeId) { t.Id = id }

func (t *CommonType) string() string { return t.Name }

func (t *CommonType) safeString(seen map[typeId]bool) string {
	return t.Name
}

func (t *CommonType) name() string { return t.Name }

// Create and check predefined types
// The string for tBytes is "bytes" not "[]byte" to signify its specialness.

var (
	// predefined because they are used by the Decoder
	tBool      typeId = 1
	tInt       typeId = 2
	tUint      typeId = 3
	tFloat     typeId = 4
	tBytes     typeId = 5
	tString    typeId = 6
	tComplex   typeId = 7
	tInterface typeId = 8
	// Reserve some Ids for compatible expansion
	tReserved7 typeId = 9
	tReserved6 typeId = 10
	tReserved5 typeId = 11
	tReserved4 typeId = 12
	tReserved3 typeId = 13
	tReserved2 typeId = 14
	tReserved1 typeId = 15
	tWireType  typeId = 16
)

var wireTypeUserInfo *userTypeInfo // userTypeInfo of (*wireType)

func init() {
	enc := NewEncoder(nil)
	builtinIdToType = make(map[typeId]gobType)
	for k, v := range enc.idToType {
		builtinIdToType[k] = v
	}

	wireTypeUserInfo = userType(reflect.TypeOf((*wireType)(nil)))
}

// Array type
type arrayType struct {
	CommonType
	Elem typeId
	Len  int
	enc *Encoder
}

func (enc *Encoder) newArrayType(name string) *arrayType {
	a := &arrayType{CommonType{Name: name}, 0, 0, enc}
	return a
}

func (a *arrayType) init(elem gobType, len int) {
	// Set our type id before evaluating the element's, in case it's our own.
	a.enc.setTypeId(a)
	a.Elem = elem.id()
	a.Len = len
}

func (a *arrayType) safeString(seen map[typeId]bool) string {
	if seen[a.Id] {
		return a.Name
	}
	seen[a.Id] = true
	return fmt.Sprintf("[%d]%s", a.Len, a.enc.gobType(a.Elem).safeString(seen))
}

func (a *arrayType) string() string { return a.safeString(make(map[typeId]bool)) }

// GobEncoder type (something that implements the GobEncoder interface)
type gobEncoderType struct {
	CommonType
	enc *Encoder
}

func (enc *Encoder) newGobEncoderType(name string) *gobEncoderType {
	g := &gobEncoderType{CommonType{Name: name}, enc}
	g.enc.setTypeId(g)
	return g
}

func (g *gobEncoderType) safeString(seen map[typeId]bool) string {
	return g.Name
}

func (g *gobEncoderType) string() string { return g.Name }

// Map type
type mapType struct {
	CommonType
	Key  typeId
	Elem typeId
	enc *Encoder
}

func (enc *Encoder) newMapType(name string) *mapType {
	m := &mapType{CommonType{Name: name}, 0, 0, enc}
	return m
}

func (m *mapType) init(key, elem gobType) {
	// Set our type id before evaluating the element's, in case it's our own.
	m.enc.setTypeId(m)
	m.Key = key.id()
	m.Elem = elem.id()
}

func (m *mapType) safeString(seen map[typeId]bool) string {
	if seen[m.Id] {
		return m.Name
	}
	seen[m.Id] = true
	key := m.enc.gobType(m.Key).safeString(seen)
	elem := m.enc.gobType(m.Elem).safeString(seen)
	return fmt.Sprintf("map[%s]%s", key, elem)
}

func (m *mapType) string() string { return m.safeString(make(map[typeId]bool)) }

// Slice type
type sliceType struct {
	CommonType
	Elem typeId
	enc *Encoder
}

func (enc *Encoder) newSliceType(name string) *sliceType {
	s := &sliceType{CommonType{Name: name}, 0, enc}
	return s
}

func (s *sliceType) init(elem gobType) {
	// Set our type id before evaluating the element's, in case it's our own.
	s.enc.setTypeId(s)
	// See the comments about ids in newTypeObject. Only slices and
	// structs have mutual recursion.
	if elem.id() == 0 {
		s.enc.setTypeId(elem)
	}
	s.Elem = elem.id()
}

func (s *sliceType) safeString(seen map[typeId]bool) string {
	if seen[s.Id] {
		return s.Name
	}
	seen[s.Id] = true
	return fmt.Sprintf("[]%s", s.enc.gobType(s.Elem).safeString(seen))
}

func (s *sliceType) string() string { return s.safeString(make(map[typeId]bool)) }

// Struct type
type fieldType struct {
	Name string
	Id   typeId
}

type structType struct {
	CommonType
	Field []*fieldType
	enc *Encoder
}

func (s *structType) safeString(seen map[typeId]bool) string {
	if s == nil {
		return "<nil>"
	}
	if _, ok := seen[s.Id]; ok {
		return s.Name
	}
	seen[s.Id] = true
	str := s.Name + " = struct { "
	for _, f := range s.Field {
		str += fmt.Sprintf("%s %s; ", f.Name, s.enc.gobType(f.Id).safeString(seen))
	}
	str += "}"
	return str
}

func (s *structType) string() string { return s.safeString(make(map[typeId]bool)) }

func (enc *Encoder) newStructType(name string) *structType {
	s := &structType{CommonType{Name: name}, nil, enc}
	// For historical reasons we set the id here rather than init.
	// See the comment in newTypeObject for details.
	s.enc.setTypeId(s)
	return s
}

// newTypeObject allocates a gobType for the reflection type rt.
// Unless ut represents a GobEncoder, rt should be the base type
// of ut.
// This is only called from the encoding side. The decoding side
// works through typeIds and userTypeInfos alone.
func (enc *Encoder) newTypeObject(name string, ut *userTypeInfo, rt reflect.Type) (gobType, error) {
	// Does this type implement GobEncoder?
	if ut.externalEnc != 0 {
		return enc.newGobEncoderType(name), nil
	}
	var err error
	var type0, type1 gobType
	defer func() {
		if err != nil {
			delete(enc.types, rt)
		}
	}()

	// Install the top-level type before the subtypes (e.g. struct before
	// fields) so recursive types can be constructed safely.
	switch t := rt; t.Kind() {
	// All basic types are easy: they are predefined.
	case reflect.Bool:
		return enc.gobType(tBool), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return enc.gobType(tInt), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return enc.gobType(tUint), nil

	case reflect.Float32, reflect.Float64:
		return enc.gobType(tFloat), nil

	case reflect.Complex64, reflect.Complex128:
		return enc.gobType(tComplex), nil

	case reflect.String:
		return enc.gobType(tString), nil

	case reflect.Interface:
		return enc.gobType(tInterface), nil

	case reflect.Array:
		at := enc.newArrayType(name)
		enc.types[rt] = at
		type0, err = enc.getBaseType("", t.Elem())
		if err != nil {
			return nil, err
		}
		// Historical aside:
		// For arrays, maps, and slices, we set the type id after the elements
		// are constructed. This is to retain the order of type id allocation after
		// a fix made to handle recursive types, which changed the order in
		// which types are built.  Delaying the setting in this way preserves
		// type ids while allowing recursive types to be described. Structs,
		// done below, were already handling recursion correctly so they
		// assign the top-level id before those of the field.
		at.init(type0, t.Len())
		return at, nil

	case reflect.Map:
		mt := enc.newMapType(name)
		enc.types[rt] = mt
		type0, err = enc.getBaseType("", t.Key())
		if err != nil {
			return nil, err
		}
		type1, err = enc.getBaseType("", t.Elem())
		if err != nil {
			return nil, err
		}
		mt.init(type0, type1)
		return mt, nil

	case reflect.Slice:
		// []byte == []uint8 is a special case
		if t.Elem().Kind() == reflect.Uint8 {
			return enc.gobType(tBytes), nil
		}
		st := enc.newSliceType(name)
		enc.types[rt] = st
		type0, err = enc.getBaseType(t.Elem().Name(), t.Elem())
		if err != nil {
			return nil, err
		}
		st.init(type0)
		return st, nil

	case reflect.Struct:
		st := enc.newStructType(name)
		enc.types[rt] = st
		enc.idToType[st.id()] = st
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !isSent(&f) {
				continue
			}
			typ := userType(f.Type).base
			tname := typ.Name()
			if tname == "" {
				t := userType(f.Type).base
				tname = t.String()
			}
			gt, err := enc.getBaseType(tname, f.Type)
			if err != nil {
				return nil, err
			}
			// Some mutually recursive types can cause us to be here while
			// still defining the element. Fix the element type id here.
			// We could do this more neatly by setting the id at the start of
			// building every type, but that would break binary compatibility.
			if gt.id() == 0 {
				enc.setTypeId(gt)
			}
			st.Field = append(st.Field, &fieldType{f.Name, gt.id()})
		}
		return st, nil

	default:
		return nil, errors.New("gob NewTypeObject can't handle type: " + rt.String())
	}
}

// isExported reports whether this is an exported - upper case - name.
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// isSent reports whether this struct field is to be transmitted.
// It will be transmitted only if it is exported and not a chan or func field
// or pointer to chan or func.
func isSent(field *reflect.StructField) bool {
	if !isExported(field.Name) {
		return false
	}
	// If the field is a chan or func or pointer thereto, don't send it.
	// That is, treat it like an unexported field.
	typ := field.Type
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Chan || typ.Kind() == reflect.Func {
		return false
	}
	return true
}

// getBaseType returns the Gob type describing the given reflect.Type's base type.
// typeLock must be held.
func (enc *Encoder) getBaseType(name string, rt reflect.Type) (gobType, error) {
	ut := userType(rt)
	return enc.getType(name, ut, ut.base)
}

// getType returns the Gob type describing the given reflect.Type.
// Should be called only when handling GobEncoders/Decoders,
// which may be pointers.  All other types are handled through the
// base type, never a pointer.
// typeLock must be held.
func (enc *Encoder) getType(name string, ut *userTypeInfo, rt reflect.Type) (gobType, error) {
	typ, present := enc.types[rt]
	if present {
		return typ, nil
	}
	typ, err := enc.newTypeObject(name, ut, rt)
	if err == nil {
		if typ == nil {
			panic("failed to create newTypeObject: " + name)
		}
		enc.types[rt] = typ
	}
	return typ, err
}

func (enc *Encoder) checkId(want, got typeId) {
	if want != got {
		fmt.Fprintf(os.Stderr, "checkId: %d should be %d\n", int(got), int(want))
		panic("bootstrap type wrong id: " + enc.name(got) + " not " + enc.name(want))
	}
}

// used for building the basic types; called only from NewEncoder.  the incoming
// interface always refers to a pointer.
func (enc *Encoder) bootstrapType(name string, e interface{}, expect typeId) typeId {
	rt := reflect.TypeOf(e).Elem()
	_, present := enc.types[rt]
	if present {
		panic("bootstrap type already present: " + name + ", " + rt.String())
	}
	typ := &CommonType{Name: name}
	enc.types[rt] = typ
	enc.setTypeId(typ)
	enc.checkId(expect, enc.nextId)
	userType(rt) // might as well cache it now
	return enc.nextId
}

// Representation of the information we send and receive about this type.
// Each value we send is preceded by its type definition: an encoded int.
// However, the very first time we send the value, we first send the pair
// (-id, wireType).
// For bootstrapping purposes, we assume that the recipient knows how
// to decode a wireType; it is exactly the wireType struct here, interpreted
// using the gob rules for sending a structure, except that we assume the
// ids for wireType and structType etc. are known.  The relevant pieces
// are built in encode.go's init() function.
// To maintain binary compatibility, if you extend this type, always put
// the new fields last.
type wireType struct {
	ArrayT           *arrayType
	SliceT           *sliceType
	StructT          *structType
	MapT             *mapType
	GobEncoderT      *gobEncoderType
	BinaryMarshalerT *gobEncoderType
	TextMarshalerT   *gobEncoderType
}

func (w *wireType) string() string {
	const unknown = "unknown type"
	if w == nil {
		return unknown
	}
	switch {
	case w.ArrayT != nil:
		return w.ArrayT.Name
	case w.SliceT != nil:
		return w.SliceT.Name
	case w.StructT != nil:
		return w.StructT.Name
	case w.MapT != nil:
		return w.MapT.Name
	case w.GobEncoderT != nil:
		return w.GobEncoderT.Name
	case w.BinaryMarshalerT != nil:
		return w.BinaryMarshalerT.Name
	case w.TextMarshalerT != nil:
		return w.TextMarshalerT.Name
	}
	return unknown
}

type typeInfo struct {
	id      typeId
	encInit sync.Mutex   // protects creation of encoder
	encoder atomic.Value // *encEngine
	wire    *wireType
}

func (enc *Encoder) lookupTypeInfo(rt reflect.Type) *typeInfo {
	m, _ := enc.typeInfoMap.Load().(map[reflect.Type]*typeInfo)
	return m[rt]
}

func (enc *Encoder) getTypeInfo(ut *userTypeInfo) (*typeInfo, error) {
	rt := ut.base
	if ut.externalEnc != 0 {
		// We want the user type, not the base type.
		rt = ut.user
	}
	if info := enc.lookupTypeInfo(rt); info != nil {
		return info, nil
	}
	return enc.buildTypeInfo(ut, rt)
}

// buildTypeInfo constructs the type information for the type
// and stores it in the type info map.
func (enc *Encoder) buildTypeInfo(ut *userTypeInfo, rt reflect.Type) (*typeInfo, error) {
	typeLock.Lock()
	defer typeLock.Unlock()

	if info := enc.lookupTypeInfo(rt); info != nil {
		return info, nil
	}

	gt, err := enc.getBaseType(rt.Name(), rt)
	if err != nil {
		return nil, err
	}
	info := &typeInfo{id: gt.id()}

	if ut.externalEnc != 0 {
		userType, err := enc.getType(rt.Name(), ut, rt)
		if err != nil {
			return nil, err
		}
		gt := enc.gobType(userType.id()).(*gobEncoderType)
		switch ut.externalEnc {
		case xGob:
			info.wire = &wireType{GobEncoderT: gt}
		case xBinary:
			info.wire = &wireType{BinaryMarshalerT: gt}
		case xText:
			info.wire = &wireType{TextMarshalerT: gt}
		}
		rt = ut.user
	} else {
		t := enc.gobType(info.id)
		switch typ := rt; typ.Kind() {
		case reflect.Array:
			info.wire = &wireType{ArrayT: t.(*arrayType)}
		case reflect.Map:
			info.wire = &wireType{MapT: t.(*mapType)}
		case reflect.Slice:
			// []byte == []uint8 is a special case handled separately
			if typ.Elem().Kind() != reflect.Uint8 {
				info.wire = &wireType{SliceT: t.(*sliceType)}
			}
		case reflect.Struct:
			info.wire = &wireType{StructT: t.(*structType)}
		}
	}

	// Create new map with old contents plus new entry.
	newm := make(map[reflect.Type]*typeInfo)
	m, _ := enc.typeInfoMap.Load().(map[reflect.Type]*typeInfo)
	for k, v := range m {
		newm[k] = v
	}
	newm[rt] = info
	enc.typeInfoMap.Store(newm)
	return info, nil
}

// Called only when a panic is acceptable and unexpected.
func (enc *Encoder) mustGetTypeInfo(rt reflect.Type) *typeInfo {
	t, err := enc.getTypeInfo(userType(rt))
	if err != nil {
		panic("getTypeInfo: " + err.Error())
	}
	return t
}

// GobEncoder is the interface describing data that provides its own
// representation for encoding values for transmission to a GobDecoder.
// A type that implements GobEncoder and GobDecoder has complete
// control over the representation of its data and may therefore
// contain things such as private fields, channels, and functions,
// which are not usually transmissible in gob streams.
//
// Note: Since gobs can be stored permanently, It is good design
// to guarantee the encoding used by a GobEncoder is stable as the
// software evolves.  For instance, it might make sense for GobEncode
// to include a version number in the encoding.
type GobEncoder interface {
	// GobEncode returns a byte slice representing the encoding of the
	// receiver for transmission to a GobDecoder, usually of the same
	// concrete type.
	GobEncode() ([]byte, error)
}

// GobDecoder is the interface describing data that provides its own
// routine for decoding transmitted values sent by a GobEncoder.
type GobDecoder interface {
	// GobDecode overwrites the receiver, which must be a pointer,
	// with the value represented by the byte slice, which was written
	// by GobEncode, usually for the same concrete type.
	GobDecode([]byte) error
}

var (
	registerLock       sync.RWMutex
	nameToConcreteType = make(map[string]reflect.Type)
	concreteTypeToName = make(map[reflect.Type]string)
)

// RegisterName is like Register but uses the provided name rather than the
// type's default.
func (enc *Encoder) RegisterName(name string, value interface{}) {
	if name == "" {
		// reserved for nil
		panic("attempt to register empty name")
	}
	registerLock.Lock()
	defer registerLock.Unlock()
	ut := userType(reflect.TypeOf(value))
	// Check for incompatible duplicates. The name must refer to the
	// same user type, and vice versa.
	if t, ok := nameToConcreteType[name]; ok && t != ut.user {
		panic(fmt.Sprintf("gob: registering duplicate types for %q: %s != %s", name, t, ut.user))
	}
	if n, ok := concreteTypeToName[ut.base]; ok && n != name {
		panic(fmt.Sprintf("gob: registering duplicate names for %s: %q != %q", ut.user, n, name))
	}
	// Store the name and type provided by the user....
	nameToConcreteType[name] = reflect.TypeOf(value)
	// but the flattened type in the type table, since that's what decode needs.
	concreteTypeToName[ut.base] = name

	// Generate gobType and assign typeId
	enc.mustGetTypeInfo(reflect.TypeOf(value))
}

// Register records a type, identified by a value for that type, under its
// internal type name.  That name will identify the concrete type of a value
// sent or received as an interface variable.  Only types that will be
// transferred as implementations of interface values need to be registered.
// Expecting to be used only during initialization, it panics if the mapping
// between types and names is not a bijection.
func (enc *Encoder) Register(value interface{}) {
	// Default to printed representation for unnamed types
	rt := reflect.TypeOf(value)
	name := rt.String()

	// But for named types (or pointers to them), qualify with import path (but see inner comment).
	// Dereference one pointer looking for a named type.
	star := ""
	if rt.Name() == "" {
		if pt := rt; pt.Kind() == reflect.Ptr {
			star = "*"
			// NOTE: The following line should be rt = pt.Elem() to implement
			// what the comment above claims, but fixing it would break compatibility
			// with existing gobs.
			//
			// Given package p imported as "full/p" with these definitions:
			//     package p
			//     type T1 struct { ... }
			// this table shows the intended and actual strings used by gob to
			// name the types:
			//
			// Type      Correct string     Actual string
			//
			// T1        full/p.T1          full/p.T1
			// *T1       *full/p.T1         *p.T1
			//
			// The missing full path cannot be fixed without breaking existing gob decoders.
			rt = pt
		}
	}
	if rt.Name() != "" {
		if rt.PkgPath() == "" {
			name = star + rt.Name()
		} else {
			name = star + rt.PkgPath() + "." + rt.Name()
		}
	}

	enc.RegisterName(name, value)
}

func (enc *Encoder) registerBasics() {
	enc.Register(int(0))
	enc.Register(int8(0))
	enc.Register(int16(0))
	enc.Register(int32(0))
	enc.Register(int64(0))
	enc.Register(uint(0))
	enc.Register(uint8(0))
	enc.Register(uint16(0))
	enc.Register(uint32(0))
	enc.Register(uint64(0))
	enc.Register(float32(0))
	enc.Register(float64(0))
	enc.Register(complex64(0i))
	enc.Register(complex128(0i))
	enc.Register(uintptr(0))
	enc.Register(false)
	enc.Register("")
	enc.Register([]byte(nil))
	enc.Register([]int(nil))
	enc.Register([]int8(nil))
	enc.Register([]int16(nil))
	enc.Register([]int32(nil))
	enc.Register([]int64(nil))
	enc.Register([]uint(nil))
	enc.Register([]uint8(nil))
	enc.Register([]uint16(nil))
	enc.Register([]uint32(nil))
	enc.Register([]uint64(nil))
	enc.Register([]float32(nil))
	enc.Register([]float64(nil))
	enc.Register([]complex64(nil))
	enc.Register([]complex128(nil))
	enc.Register([]uintptr(nil))
	enc.Register([]bool(nil))
	enc.Register([]string(nil))
}
