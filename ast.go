/*
   Copyright 2021 Lee R. Boynton

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
package smithy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/boynton/data"
)

const UnspecifiedNamespace = "example"
const UnspecifiedVersion = "0.0"

type AST struct {
	Smithy   string       `json:"smithy"`
	Metadata *data.Object `json:"metadata,omitempty"`
	Shapes   *Shapes      `json:"shapes,omitempty"`
}

func (ast *AST) AssemblyVersion() int {
	if strings.HasPrefix(ast.Smithy, "1") {
		return 1
	}
	return 2
}

// a Shapes object is a map from Shape ID to *Shape. It preserves the order of its keys, unlike a Go map
type Shapes struct {
	keys     []string
	bindings map[string]*Shape
}

func NewShapes() *Shapes {
	return &Shapes{
		bindings: make(map[string]*Shape, 0),
	}
}

func (s *Shapes) UnmarshalJSON(raw []byte) error {
	keys, err := data.JsonKeysInOrder(raw)
	if err != nil {
		return err
	}
	shapes := NewShapes()
	shapes.keys = keys
	err = json.Unmarshal(raw, &shapes.bindings)
	if err != nil {
		return err
	}
	*s = *shapes
	return nil
}

func (s Shapes) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("{")
	for i, key := range s.keys {
		value := s.bindings[key]
		if i > 0 {
			buffer.WriteString(",")
		}
		jsonValue, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		buffer.WriteString(fmt.Sprintf("%q:%s", key, string(jsonValue)))
	}
	buffer.WriteString("}")
	return buffer.Bytes(), nil
}

func (s *Shapes) Put(key string, val *Shape) {
	if _, ok := s.bindings[key]; !ok {
		s.keys = append(s.keys, key)
	}
	s.bindings[key] = val
}

func (s *Shapes) Get(key string) *Shape {
	return s.bindings[key]
}

func (s *Shapes) Keys() []string {
	return s.keys
}

func (s *Shapes) Length() int {
	if s == nil || s.keys == nil {
		return 0
	}
	return len(s.keys)
}

func (ast *AST) PutShape(id string, shape *Shape) {
	if ast.Shapes == nil {
		ast.Shapes = NewShapes()
	}
	ast.Shapes.Put(id, shape)
}

func (ast *AST) GetShape(id string) *Shape {
	if ast.Shapes == nil {
		return nil
	}
	return ast.Shapes.Get(id)
}

// a Members object is a map from string to *Member. It preserves the order of its keys, unlike a Go map
type Members struct {
	keys     []string
	bindings map[string]*Member
}

func NewMembers() *Members {
	return &Members{
		bindings: make(map[string]*Member, 0),
	}
}

func (m *Members) UnmarshalJSON(raw []byte) error {
	keys, err := data.JsonKeysInOrder(raw)
	if err != nil {
		return err
	}
	members := NewMembers()
	members.keys = keys
	err = json.Unmarshal(raw, &members.bindings)
	if err != nil {
		return err
	}
	*m = *members
	return nil
}

func (m Members) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("{")
	for i, key := range m.keys {
		value := m.bindings[key]
		if i > 0 {
			buffer.WriteString(",")
		}
		jsonValue, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		buffer.WriteString(fmt.Sprintf("%q:%s", key, string(jsonValue)))
	}
	buffer.WriteString("}")
	return buffer.Bytes(), nil
}

func (m *Members) Put(key string, val *Member) {
	if _, ok := m.bindings[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.bindings[key] = val
}

func (m *Members) Get(key string) *Member {
	return m.bindings[key]
}

func (m *Members) Keys() []string {
	if m != nil {
		return m.keys
	}
	return nil
}

func (m *Members) Length() int {
	if m == nil || m.keys == nil {
		return 0
	}
	return len(m.keys)
}

type Shape struct {
	Type   string       `json:"type"`
	Traits *data.Object `json:"traits,omitempty"` //service, resource, operation, apply

	//List and Set
	Member *Member `json:"member,omitempty"`

	//Map
	Key   *Member `json:"key,omitempty"`
	Value *Member `json:"value,omitempty"`

	//Structure and Union
	Members *Members    `json:"members,omitempty"` //keys must be case-insensitively unique. For union, len(Members) > 0,
	Mixins  []*ShapeRef `json:"mixins,omitempty"`  //mixins for the shape

	//Resource
	Identifiers map[string]*ShapeRef `json:"identifiers,omitempty"`
	//FIXME preserve resource identifier order?
	Create               *ShapeRef   `json:"create,omitempty"`
	Put                  *ShapeRef   `json:"put,omitempty"`
	Read                 *ShapeRef   `json:"read,omitempty"`
	Update               *ShapeRef   `json:"update,omitempty"`
	Delete               *ShapeRef   `json:"delete,omitempty"`
	List                 *ShapeRef   `json:"list,omitempty"`
	CollectionOperations []*ShapeRef `json:"collectionOperations,omitempty"`

	//Resource and Service
	Operations []*ShapeRef `json:"operations,omitempty"`
	Resources  []*ShapeRef `json:"resources,omitempty"`

	//Operation
	Input  *ShapeRef   `json:"input,omitempty"`
	Output *ShapeRef   `json:"output,omitempty"`
	Errors []*ShapeRef `json:"errors,omitempty"`

	//Service
	Version string `json:"version,omitempty"`
}

type ShapeRef struct {
	Target string `json:"target"`
}

type Member struct {
	Target string       `json:"target"`
	Traits *data.Object `json:"traits,omitempty"`
}

func shapeIdNamespace(id string) string {
	//name.space#entity$member
	lst := strings.Split(id, "#")
	return lst[0]
}

func (ast *AST) Validate() error {
	//todo
	return nil
}

func (ast *AST) Namespaces() []string {
	m := make(map[string]int, 0)
	if ast.Shapes != nil {
		for _, id := range ast.Shapes.Keys() {
			ns := shapeIdNamespace(id)
			if n, ok := m[ns]; ok {
				m[ns] = n + 1
			} else {
				m[ns] = 1
			}
		}
	}
	nss := make([]string, 0, len(m))
	for k, _ := range m {
		nss = append(nss, k)
	}
	return nss
}

func (ast *AST) RequiresDocumentType() bool {
	included := make(map[string]bool, 0)
	for _, k := range ast.Shapes.Keys() {
		ast.noteDependencies(included, k)
	}
	if _, ok := included["smithy.api#Document"]; ok {
		return true
	}
	return false
}

func (ast *AST) noteDependenciesFromRef(included map[string]bool, ref *ShapeRef) {
	if ref != nil {
		ast.noteDependencies(included, ref.Target)
	}
}

func (ast *AST) noteDependencies(included map[string]bool, name string) {
	//note traits
	if name == "smithy.api#Document" {
		included[name] = true
		return
	}
	if name == "" || strings.HasPrefix(name, "smithy.api#") {
		return
	}
	if _, ok := included[name]; ok {
		return
	}
	included[name] = true
	shape := ast.GetShape(name)
	if shape == nil {
		return
	}
	if shape.Traits != nil {
		for _, tk := range shape.Traits.Keys() {
			ast.noteDependencies(included, tk)
		}
	}
	switch shape.Type {
	case "operation":
		ast.noteDependenciesFromRef(included, shape.Input)
		ast.noteDependenciesFromRef(included, shape.Output)
		for _, e := range shape.Errors {
			ast.noteDependenciesFromRef(included, e)
		}
	case "resource":
		if shape.Identifiers != nil {
			for _, v := range shape.Identifiers {
				ast.noteDependenciesFromRef(included, v)
			}
		}
		for _, o := range shape.Operations {
			ast.noteDependenciesFromRef(included, o)
		}
		for _, r := range shape.Resources {
			ast.noteDependenciesFromRef(included, r)
		}
		ast.noteDependenciesFromRef(included, shape.Create)
		ast.noteDependenciesFromRef(included, shape.Put)
		ast.noteDependenciesFromRef(included, shape.Read)
		ast.noteDependenciesFromRef(included, shape.Update)
		ast.noteDependenciesFromRef(included, shape.Delete)
		ast.noteDependenciesFromRef(included, shape.List)
		for _, o := range shape.CollectionOperations {
			ast.noteDependenciesFromRef(included, o)
		}
	case "structure", "union":
		for _, n := range shape.Members.Keys() {
			m := shape.Members.Get(n)
			ast.noteDependencies(included, m.Target)
		}
	case "list", "set":
		ast.noteDependencies(included, shape.Member.Target)
	case "map":
		ast.noteDependencies(included, shape.Key.Target)
		ast.noteDependencies(included, shape.Value.Target)
	case "string", "integer", "long", "short", "byte", "float", "double", "boolean", "bigInteger", "bigDecimal", "blob", "timestamp":
		//smithy primitives
	}
}

func (ast *AST) ShapeNames() []string {
	var lst []string
	for _, k := range ast.Shapes.Keys() {
		lst = append(lst, k)
	}
	return lst
}

func LoadAST(path string) (*AST, error) {
	var ast *AST
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Cannot read smithy AST file: %v\n", err)
	}
	err = json.Unmarshal(data, &ast)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse Smithy AST file: %v\n", err)
	}
	if ast.Smithy == "" {
		return nil, fmt.Errorf("Cannot parse Smithy AST file: %v\n", err)
	}
	return ast, nil
}

func (ast *AST) Merge(src *AST) error {
	if ast.Smithy != src.Smithy {
		if strings.HasPrefix(ast.Smithy, "1") && strings.HasPrefix(src.Smithy, "2") {
			ast.Smithy = src.Smithy
		} else {
			fmt.Println("//WARNING: smithy version mismatch:", ast.Smithy, "and", src.Smithy)
		}
	}
	if src.Metadata != nil {
		if ast.Metadata == nil {
			ast.Metadata = src.Metadata
		} else {
			for _, k := range src.Metadata.Keys() {
				v := src.Metadata.Get(k)
				prev := ast.Metadata.Get(k)
				if prev != nil {
					err := ast.mergeConflict(k, prev, v)
					if err != nil {
						return err
					}
				}
				ast.Metadata.Put(k, v)
			}
		}
	}
	if src.Shapes != nil {
		for _, k := range src.Shapes.Keys() {
			if tmp := ast.GetShape(k); tmp != nil {
				return fmt.Errorf("Duplicate shape in assembly: %s\n", k)
			}
			ast.PutShape(k, src.GetShape(k))
		}
	}
	return nil
}

func (ast *AST) mergeConflict(k string, v1 interface{}, v2 interface{}) error {
	//todo: if values are identical, accept one of them
	//todo: concat list values
	return fmt.Errorf("Conflict when merging metadata in models: %s\n", k)
}

func (ast *AST) Filter(tags []string) {
	var root []string
	for _, k := range ast.Shapes.Keys() {
		shape := ast.Shapes.Get(k)
		shapeTags := shape.Traits.GetStringArray("smithy.api#tags")
		if shapeTags != nil {
			for _, t := range shapeTags {
				if containsString(tags, t) {
					root = append(root, k)
				}
			}
		}
	}
	included := make(map[string]bool, 0)
	for _, k := range root {
		if _, ok := included[k]; !ok {
			ast.noteDependencies(included, k)
		}
	}
	filtered := NewShapes()
	for name, _ := range included {
		if !strings.HasPrefix(name, "smithy.api#") {
			filtered.Put(name, ast.GetShape(name))
		}
	}
	ast.Shapes = filtered
}

func containsString(ary []string, val string) bool {
	for _, s := range ary {
		if s == val {
			return true
		}
	}
	return false
}
