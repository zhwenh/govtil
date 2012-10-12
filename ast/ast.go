package ast

import (
	"crypto/sha1"
	"fmt"
	stdast "go/ast"
	"hash"
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

func hashAST(n stdast.Node) ([]byte, error) {
	h := sha1.New()
	ids := &objManager{make(map[*stdast.Object][]byte), 0}
	if err := doHashAST(n, h, ids); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

type nameIndex struct {
	name string
	idx  int
}

type nameIndexes []*nameIndex

func (n nameIndexes) Len() int           { return len(n) }
func (n nameIndexes) Less(i, j int) bool { return n[i].name < n[j].name }
func (n nameIndexes) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }

func doHashAST(n stdast.Node, h hash.Hash, objs *objManager) error {
	h.Write([]byte(fmt.Sprint(reflect.TypeOf(n).Elem().Name())))

	switch v := n.(type) {

	case *stdast.File:
		// imports (sort then hash since import order doesn't matter)
		paths := make([]string, 0)
		for _, v2 := range v.Imports {
			paths = append(paths, v2.Path.Value)
		}
		var spaths sort.StringSlice = paths
		spaths.Sort()
		for _, p := range paths {
			h.Write([]byte(p))
		}

		// declarations
		for _, v2 := range v.Decls {
			if err := doHashAST(v2, h, objs); err != nil {
				return err
			}
		}

	case *stdast.GenDecl:
		for _, v2 := range v.Specs {
			if err := doHashAST(v2, h, objs); err != nil {
				return err
			}
		}

	case *stdast.ImportSpec:
		// handled in *stdast.File
		break

	case *stdast.ValueSpec:
		// has n names, a type and n values (matched by indexes)
		// 1) hash type
		// 2) sort names
		// 3) hash each name and its value in sorted order
		if err := doHashAST(v.Type, h, objs); err != nil {
			return err
		}
		names := make(nameIndexes, 0)
		for i, n := range v.Names {
			names = append(names, &nameIndex{n.Name, i})
		}
		sort.Sort(names)
		for _, ni := range names {
			if err := doHashAST(v.Names[ni.idx], h, objs); err != nil {
				return err
			}
			if err := doHashAST(v.Values[ni.idx], h, objs); err != nil {
				return err
			}
		}

	case *stdast.Ident:
		if v.Obj != nil {
			h.Write(objs.Hash(v.Obj))
		} else {
			h.Write([]byte(v.Name))
		}

	case *stdast.BasicLit:
		h.Write([]byte(fmt.Sprint(v.Value)))

	case *stdast.CompositeLit:
		if err := doHashAST(v.Type, h, objs); err != nil {
			return err
		}
		for _, e := range v.Elts {
			if err := doHashAST(e, h, objs); err != nil {
				return err
			}
		}

	case *stdast.ParenExpr:
		if err := doHashAST(v.X, h, objs); err != nil {
			return err
		}

	case *stdast.FuncDecl:
		if v.Recv != nil {
			if err := doHashAST(v.Recv, h, objs); err != nil {
				return err
			}
		}
		h.Write(objs.Hash(v.Name.Obj))
		if err := doHashAST(v.Type, h, objs); err != nil {
			return err
		}
		// body
		if err := doHashAST(v.Body, h, objs); err != nil {
			return err
		}

	case *stdast.FuncType:
		// params
		if v.Params != nil {
			if err := doHashAST(v.Params, h, objs); err != nil {
				return err
			}
		}
		if v.Results != nil {
			if err := doHashAST(v.Results, h, objs); err != nil {
				return err
			}
		}

	case *stdast.FieldList:
		for _, f := range v.List {
			for _, n := range f.Names {
				h.Write(objs.Hash(n.Obj))
			}
			// hash count and type
			h.Write([]byte(fmt.Sprint("%d", len(f.Names))))
			if err := doHashAST(f.Type, h, objs); err != nil {
				return err
			}
		}

	case *stdast.BlockStmt:
		for _, s := range v.List {
			if err := doHashAST(s, h, objs); err != nil {
				return err
			}
		}

	case *stdast.ExprStmt:
		if err := doHashAST(v.X, h, objs); err != nil {
			return err
		}

	case *stdast.IncDecStmt:
		if err := doHashAST(v.X, h, objs); err != nil {
			return err
		}
		h.Write([]byte(fmt.Sprint(v.Tok)))

	case *stdast.CallExpr:
		if err := doHashAST(v.Fun, h, objs); err != nil {
			return err
		}
		for _, a := range v.Args {
			if err := doHashAST(a, h, objs); err != nil {
				return err
			}
		}

	case *stdast.UnaryExpr:
		h.Write([]byte(fmt.Sprint(v.Op)))
		if err := doHashAST(v.X, h, objs); err != nil {
			return err
		}

	case *stdast.BinaryExpr:
		if err := doHashAST(v.X, h, objs); err != nil {
			return err
		}
		h.Write([]byte(fmt.Sprint(v.Op)))
		if err := doHashAST(v.Y, h, objs); err != nil {
			return err
		}

	case *stdast.SelectorExpr:
		if err := doHashAST(v.X, h, objs); err != nil {
			return err
		}
		if err := doHashAST(v.Sel, h, objs); err != nil {
			return err
		}

	case *stdast.StarExpr:
		if err := doHashAST(v.X, h, objs); err != nil {
			return err
		}

	case *stdast.IndexExpr:
		if err := doHashAST(v.X, h, objs); err != nil {
			return err
		}
		if err := doHashAST(v.Index, h, objs); err != nil {
			return err
		}

	case *stdast.TypeAssertExpr:
		if err := doHashAST(v.X, h, objs); err != nil {
			return err
		}

	case *stdast.IfStmt:
		if err := doHashAST(v.Cond, h, objs); err != nil {
			return err
		}
		if err := doHashAST(v.Body, h, objs); err != nil {
			return err
		}

	case *stdast.AssignStmt:
		for _, e := range v.Lhs {
			if err := doHashAST(e, h, objs); err != nil {
				return err
			}
		}
		h.Write([]byte(fmt.Sprint(v.Tok)))
		for _, e := range v.Rhs {
			if err := doHashAST(e, h, objs); err != nil {
				return err
			}
		}

	case *stdast.TypeSwitchStmt:
		if err := doHashAST(v.Assign, h, objs); err != nil {
			return err
		}
		if err := doHashAST(v.Body, h, objs); err != nil {
			return err
		}

	case *stdast.CaseClause:
		for _, e := range v.List {
			if err := doHashAST(e, h, objs); err != nil {
				return err
			}
		}
		for _, s := range v.Body {
			if err := doHashAST(s, h, objs); err != nil {
				return err
			}
		}

	case *stdast.RangeStmt:
		if err := doHashAST(v.Key, h, objs); err != nil {
			return err
		}
		if err := doHashAST(v.Value, h, objs); err != nil {
			return err
		}
		if err := doHashAST(v.X, h, objs); err != nil {
			return err
		}
		if err := doHashAST(v.Body, h, objs); err != nil {
			return err
		}

	case *stdast.DeclStmt:
		if err := doHashAST(v.Decl, h, objs); err != nil {
			return err
		}

	case *stdast.BranchStmt:
		h.Write([]byte(fmt.Sprint(v.Tok)))

	case *stdast.ReturnStmt:
		for _, r := range v.Results {
			if err := doHashAST(r, h, objs); err != nil {
				return err
			}
		}

	case *stdast.TypeSpec:
		if err := doHashAST(v.Name, h, objs); err != nil {
			return err
		}
		if err := doHashAST(v.Type, h, objs); err != nil {
			return err
		}

	case *stdast.StructType:
		if err := doHashAST(v.Fields, h, objs); err != nil {
			return err
		}
		if v.Incomplete {
			h.Write([]byte("Incomplete"))
		}

	case *stdast.MapType:
		if err := doHashAST(v.Key, h, objs); err != nil {
			return err
		}
		if err := doHashAST(v.Value, h, objs); err != nil {
			return err
		}

	case *stdast.ArrayType:
		if err := doHashAST(v.Elt, h, objs); err != nil {
			return err
		}

	default:
		return fmt.Errorf("hastAST: unknown type *%s", reflect.TypeOf(n).Elem().Name())
	}
	return nil
}
