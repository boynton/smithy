package smithy

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/boynton/smithy/data"
)

const SmithyVersion = "1.0"
const UnspecifiedNamespace = "example"
const UnspecifiedVersion = "0.0"

type AST struct {
	Smithy   string       `json:"smithy"`
	Metadata *data.Object `json:"metadata,omitempty"`
	Shapes   *Shapes      `json:"shapes,omitempty"`
}

// a Shapes object is a map from Shape ID to *Shape. It preserves the order of its keys, unlike a Go map
type Shapes struct {
	keys     []string
	bindings map[string]*Shape
}

func newShapes() *Shapes {
	return &Shapes{
		bindings: make(map[string]*Shape, 0),
	}
}

func (s *Shapes) UnmarshalJSON(raw []byte) error {
	keys, err := data.JsonKeysInOrder(raw)
	if err != nil {
		return err
	}
	shapes := newShapes()
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
		ast.Shapes = newShapes()
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

func newMembers() *Members {
	return &Members{
		bindings: make(map[string]*Member, 0),
	}
}

func (m *Members) UnmarshalJSON(raw []byte) error {
	keys, err := data.JsonKeysInOrder(raw)
	if err != nil {
		return err
	}
	members := newMembers()
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
	Members *Members `json:"members,omitempty"` //keys must be case-insensitively unique. For union, len(Members) > 0,

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
