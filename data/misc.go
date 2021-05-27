package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func Kind(v interface{}) string {
	return fmt.Sprintf("%v", reflect.ValueOf(v).Kind())
}

func Equivalent(obj1 interface{}, obj2 interface{}) bool {
	return Pretty(obj1) == Pretty(obj2)
}

func Pretty(obj interface{}) string {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&obj); err != nil {
		return fmt.Sprint(obj)
	}
	return string(buf.String())
}

func Json(obj interface{}) string {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&obj); err != nil {
		return fmt.Sprint(obj)
	}
	return strings.TrimRight(string(buf.String()), " \t\n\v\f\r")
}
