package smithy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type Struct struct {
	keys     []string
	Bindings map[string]interface{}
}

func jsonKeysInOrder(data []byte) ([]string, error) {
	var end = fmt.Errorf("invalid end of array or object")

	var skipValue func(d *json.Decoder) error
	skipValue = func(d *json.Decoder) error {
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('['), json.Delim('{'):
			for {
				if err := skipValue(d); err != nil {
					if err == end {
						break
					}
					return err
				}
			}
		case json.Delim(']'), json.Delim('}'):
			return end
		}
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	if t != json.Delim('{') {
		return nil, fmt.Errorf("expected start of object")
	}
	var keys []string
	for {
		t, err := d.Token()
		if err != nil {
			return nil, err
		}
		if t == json.Delim('}') {
			return keys, nil
		}
		keys = append(keys, t.(string))
		if err := skipValue(d); err != nil {
			return nil, err
		}
	}
}

func (s *Struct) UnmarshalJSON(data []byte) error {
	keys, err := jsonKeysInOrder(data)
	if err != nil {
		return err
	}
	str := NewStruct()
	str.keys = keys
	err = json.Unmarshal(data, &str.Bindings)
	if err != nil {
		return err
	}
	*s = *str
	return nil
}

func (s *Struct) String() string {
	return Json(s)
}

func (s Struct) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("{")
	for i, key := range s.keys {
		value := s.Bindings[key]
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

func NewStruct() *Struct {
	return &Struct{
		Bindings: make(map[string]interface{}, 0),
	}
}

/*

func FromStruct(s *Struct) map[string]interface{} {
	if s == nil {
		return nil
	}
	return s.Bindings
}

func ToStruct(m map[string]interface{}) *Struct {
	result := &Struct{Bindings: m}
	for k, _ := range m { //bug: cannot preserve the order
		result.keys = append(result.keys, k)
	}
	return result
}
*/

func (s *Struct) find(key string) int {
	for i, k := range s.keys {
		if k == key {
			return i
		}
	}
	return -1
}

func (s *Struct) Has(key string) bool {
	if _, ok := s.Bindings[key]; ok {
		return true
	}
	return false
}

func (s *Struct) Get(key string) interface{} {
	return s.Bindings[key]
}

func (s *Struct) Put(key string, val interface{}) {
	if _, ok := s.Bindings[key]; !ok {
		s.keys = append(s.keys, key)
	}
	s.Bindings[key] = val
}

func (s *Struct) Keys() []string {
	return s.keys
}

func (s *Struct) Length() int {
	if s == nil || s.keys == nil {
		return 0
	}
	return len(s.keys)
}

func ToString(obj interface{}) string {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&obj); err != nil {
		return fmt.Sprint(obj)
	}
	s := buf.String()
	s = strings.Trim(s, " \n")
	return string(s)
}

func AsMap(v interface{}) map[string]interface{} {
	if v != nil {
		switch m := v.(type) {
		case map[string]interface{}:
			return m
		case *Struct:
			return m.Bindings
		}
	}
	return nil
}

func AsArray(v interface{}) []interface{} {
	if v != nil {
		if a, ok := v.([]interface{}); ok {
			return a
		}
	}
	return nil
}

func AsStringArray(v interface{}) []string {
	var sa []string
	a := AsArray(v)
	if a != nil {
		for _, i := range a {
			switch s := i.(type) {
			case *string:
				sa = append(sa, *s)
			case string:
				sa = append(sa, s)
			default:
				return nil
			}
		}
	}
	return sa
}

func AsString(v interface{}) string {
	if v != nil {
		switch s := v.(type) {
		case string:
			return s
		case *string:
			return *s
		}
	}
	return ""
}

func AsBool(v interface{}) bool {
	if v != nil {
		if b, isBool := v.(bool); isBool {
			return b
		}
		return true
	}
	return false
}

func AsInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int32:
		return int(n)
	case int64:
		return int(n)
	case int:
		return n
	case *Decimal:
		return n.AsInt()
	}
	return 0
}

func AsInt64(v interface{}) int64 {
	if n, ok := v.(float64); ok {
		return int64(n)
	}
	return 0
}

func AsFloat64(v interface{}) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return 0
}

func AsDecimal(v interface{}) *Decimal {
	switch n := v.(type) {
	case Decimal:
		return &n
	case *Decimal:
		return n
	default:
		return nil
	}
}

func Get(m map[string]interface{}, key string) interface{} {
	if m != nil {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func GetString(m map[string]interface{}, key string) string {
	return AsString(Get(m, key))
}
func GetStringArray(m map[string]interface{}, key string) []string {
	return AsStringArray(Get(m, key))
}
func GetBool(m map[string]interface{}, key string) bool {
	return AsBool(Get(m, key))
}
func GetInt(m map[string]interface{}, key string) int {
	return AsInt(Get(m, key))
}
func GetInt64(m map[string]interface{}, key string) int64 {
	return AsInt64(Get(m, key))
}
func GetArray(m map[string]interface{}, key string) []interface{} {
	return AsArray(Get(m, key))
}
func GetMap(m map[string]interface{}, key string) map[string]interface{} {
	return AsMap(Get(m, key))
}
func GetDecimal(m map[string]interface{}, key string) *Decimal {
	return AsDecimal(Get(m, key))
}
