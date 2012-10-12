package ast

import (
	"bytes"
	stdast "go/ast"
	"go/parser"
	"go/token"
	//	"os"
	"testing"
)

var data []byte = []byte(`
package test
import "fmt"
import "errors"

var st string = "dingo"
var it int = (4 + 8)

var s1, s2 string = "string1", "string2"

func blah(j int, _ string) int {
	fmt.Println("blah")
	fmt.Println(j)
	if j > 6 {
	errors.New("blah")
	}
	return 6
}

func caller() {
	blah(1, "?")
}
`)

func TestAST(t *testing.T) {
	b := bytes.NewBuffer(data)
	//b, _ := os.Open("ast.go")
	fset := token.NewFileSet()
	tree, err := parser.ParseFile(fset, "", b, 0)
	if err != nil {
		t.Fatal(err)
	}
	stdast.Print(fset, tree)
	hash, err := hashAST(tree)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%x", hash)
}
