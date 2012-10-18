package ast

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	stdast "go/ast"
	"reflect"
	"sort"
)

type objManager struct {
	objs map[*stdast.Object][]byte
	i    int
}

func (im *objManager) Hash(o *stdast.Object) []byte {
	r, ok := im.objs[o]
	if !ok {
		r = []byte(fmt.Sprint(im.i))
		im.objs[o] = r
		im.i++
	}
	return r
}

func Hash(n *stdast.File) ([]byte, error) {
	hv := &hashVisitor{
		hashes: [][]byte{},
		parent: nil,
		err:    nil,
	}
	stdast.Walk(hv, n)
	if hv.err != nil {
		return nil, hv.err
	}
	return hv.Sum(nil), nil
}

type byteSliceSlice [][]byte

func (bss byteSliceSlice) Len() int {
	return len(bss)
}

func (bss byteSliceSlice) Less(i, j int) bool {
	return bytes.Compare(bss[i], bss[j]) == -1
}

func (bss byteSliceSlice) Swap(i, j int) {
	bss[i], bss[j] = bss[j], bss[i]
}

// hashVisitor implements a tree visitor that performs an ordered or unordered
// hash over a collection of sub elements
type hashVisitor struct {
	hashes  byteSliceSlice
	parent  *hashVisitor
	err     error
	ordered bool
}

func (hv *hashVisitor) add(b []byte) {
	hv.hashes = append(hv.hashes, b)
}

func (hv *hashVisitor) addString(s string) {
	hv.add([]byte(s))
}

func (hv *hashVisitor) setError(err error) {
	hv.err = err
}

const (
	ordered = iota
	unordered
)

func (hv *hashVisitor) Visit(n stdast.Node) stdast.Visitor {
	// terminate if no more children (n==nil) or error
	if n == nil || hv.err != nil {
		if hv.parent != nil {
			hv.parent.err = hv.err
			hv.parent.add(hv.Sum(nil))
		}
		return nil
	}

	// hash myself
	hv.addString(fmt.Sprint(reflect.TypeOf(n).Elem().Name()))
	orderedChildren := false
	switch v := n.(type) {

	// these tree elements have some internal content in addition to their
	// children
	case *stdast.AssignStmt:
		hv.addString(fmt.Sprint(v.Tok))
		orderedChildren = true

	case *stdast.BasicLit:
		hv.addString(fmt.Sprint(v.Value))

	case *stdast.BinaryExpr:
		hv.addString(fmt.Sprint(v.Op))
		orderedChildren = true

	case *stdast.BranchStmt:
		hv.addString(fmt.Sprint(v.Tok))

	case *stdast.ChanType:
		hv.addString(fmt.Sprint(v.Dir))

	case *stdast.GenDecl:
		hv.addString(fmt.Sprint(v.Tok))

	case *stdast.Ident:
		// TODO(vsekhar): match by identity via v.Obj (*stdast.Object)
		hv.addString(v.Name)

	// these tree elements have no content other than their children
	case *stdast.ArrayType:
	case *stdast.BlockStmt:
		orderedChildren = true
	case *stdast.CallExpr:
		orderedChildren = true
	case *stdast.CaseClause:
		orderedChildren = true // though only for the statements...
	case *stdast.CommClause:
		orderedChildren = true
	case *stdast.CompositeLit:
		orderedChildren = true
	case *stdast.DeclStmt:
	case *stdast.Ellipsis:
	case *stdast.EmptyStmt:
	case *stdast.ExprStmt:
	case *stdast.Field:
	case *stdast.FieldList:
		// for function signatures and non-named initialisations
		orderedChildren = true
	case *stdast.File:
	case *stdast.ForStmt:
		orderedChildren = true
	case *stdast.FuncDecl:
		orderedChildren = true
	case *stdast.FuncType:
	case *stdast.GoStmt:
	case *stdast.IfStmt:
		orderedChildren = true
	case *stdast.ImportSpec:
	case *stdast.IncDecStmt:
	case *stdast.IndexExpr:
		// consider an indexable type indexed by another indexable type,
		// they could feasibly swap places, with different symantics
		orderedChildren = true
	case *stdast.InterfaceType: // incomplete == error
	case *stdast.KeyValueExpr:
		orderedChildren = true
	case *stdast.MapType:
		orderedChildren = true
	case *stdast.ParenExpr:
		orderedChildren = true
	case *stdast.RangeStmt:
		orderedChildren = true
	case *stdast.ReturnStmt:
		orderedChildren = true
	case *stdast.SelectorExpr:
		orderedChildren = true
	case *stdast.SendStmt:
		orderedChildren = true
	case *stdast.SliceExpr:
		orderedChildren = true
	case *stdast.StarExpr:
	case *stdast.StructType: // incomplete == error
	case *stdast.TypeAssertExpr:
		orderedChildren = true
	case *stdast.TypeSpec:
	case *stdast.TypeSwitchStmt:
		orderedChildren = true
	case *stdast.UnaryExpr:
	case *stdast.ValueSpec:
		orderedChildren = true

	default:
		msg := fmt.Sprintf("hashAST: unknown type *%s, terminated early", reflect.TypeOf(n).Elem().Name())
		hv.err = errors.New(msg)
		return nil
	}

	// spawn a visitor for my children, pointing back to me
	return &hashVisitor{
		hashes:  [][]byte{},
		parent:  hv,
		err:     nil,
		ordered: orderedChildren,
	}
}

func (hv *hashVisitor) Sum(b []byte) []byte {
	if !hv.ordered {
		sort.Sort(hv.hashes)
	}
	h := sha1.New()
	for _, curHash := range hv.hashes {
		h.Write(curHash)
	}
	return h.Sum(b)
}
