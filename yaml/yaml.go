// Package yaml provides helpers for traversing a YAML file represented as a
// tree of nodes.
package yaml

import (
	"fmt"
	"io/ioutil"
	"os"

	yaml "launchpad.net/goyaml"
)

type Node struct {
	Value interface{}
}

func read(filename string, dest interface{}) (n *Node, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(bytes, dest)
	if err != nil {
		return nil, err
	}
	return &Node{dest}, nil
}

// Read filename, assuming it is structured as a list at the top level.
func ReadAsList(filename string) (*Node, error) {
	l := make([]interface{}, 0)
	return read(filename, &l)
}

// Read filename, assuming it is structured as a map at the top level.
func ReadAsMap(filename string) (*Node, error) {
	m := make(map[interface{}]interface{})
	return read(filename, &m)
}

// Return true if the node is a list
func (n *Node) IsList() bool {
	_, ok := n.Value.(*[]interface{})
	return ok
}

// Convert node to a []*Node
func (n *Node) AsList() []*Node {
	l := n.Value.(*[]interface{})
	ret := make([]*Node, 0, len(*l))
	for _, li := range *l {
		ret = append(ret, &Node{li})
	}
	return ret
}

// Get i-th child node, assuming current node is a list 
func (n *Node) At(i int) *Node {
	l := n.Value.(*[]interface{})
	return &Node{(*l)[i]}
}

// Return true if the node is a map
func (n *Node) IsMap() bool {
	_, ok := n.Value.(map[interface{}]interface{})
	return ok
}

// Convert node to a map[string]*Node
func (n *Node) AsMap() map[string]*Node {
	m := n.Value.(map[interface{}]interface{})
	ret := make(map[string]*Node)
	for k, v := range m {
		ret[k.(string)] = &Node{v}
	}
	return ret
}

// Get the node corresponding to key k, assuming current node is a map
func (n *Node) Key(k string) *Node {
	m := n.Value.(map[interface{}]interface{})
	return &Node{m[k]}
}

// Get a string representation of the current node and all its children
func (n *Node) String() string {
	return fmt.Sprint(n.Value)
}
