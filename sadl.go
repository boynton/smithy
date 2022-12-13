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
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/boynton/data"
)

type SadlGenerator struct {
	BaseGenerator
}

func (gen *SadlGenerator) Generate(ast *AST, config *data.Object) error {
	err := gen.Configure(config)
	if err != nil {
		return err
	}
	ns := config.GetString("namespace")
	fbase := ns
	if fbase == "" {
		fbase = "model"
	}
	fname := gen.FileName(fbase, ".sadl")
	err = gen.Validate(ns, ast)
	if err != nil {
		return err
	}
	s := gen.ToSadl(ns, ast)
	return gen.Emit(s, fname, "")
}

func (gen *SadlGenerator) Validate(ns string, ast *AST) error {
	for _, nsk := range ast.Shapes.Keys() {
		shape := ast.GetShape(nsk)
		if shape == nil {
			return fmt.Errorf("Undefined shape: %s\n", nsk)
		}
		lst := strings.Split(nsk, "#")
		k := lst[1]
		if shape.Type == "operation" {
			err := gen.validateOperation(lst[0], k, shape, ast)
			if err != nil {
				return err
			}
		} else {
			err := gen.validateType(lst[0], k, shape, ast)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (gen *SadlGenerator) validateType(ns, n string, shape *Shape, ast *AST) error {
	switch shape.Type {
	case "intEnum":
		return fmt.Errorf("intEnum not supported by SADL: %s#%s", ns, n)
	}
	return nil
}

func (gen *SadlGenerator) validateOperation(ns, n string, shape *Shape, ast *AST) error {
	fullName := ns + "#" + n
	httpTrait := shape.Traits.GetObject("smithy.api#http")
	if httpTrait == nil {
		return fmt.Errorf("Operation without @http trait not valid for SADL: %s", fullName)
	}
	method := httpTrait.GetString("method")
	expectInputPayload := method == "PUT" || method == "POST" || method == "PATCH"
	inputPayload := false
	if shape.Input != nil {
		inShape := ast.GetShape(shape.Input.Target)
		if inShape == nil {
			return fmt.Errorf("Undefined shape: %s\n", shape.Input.Target)
		}
		for _, k := range inShape.Members.Keys() {
			var isPayload, isHeader, isQuery, isLabel bool
			v := inShape.Members.Get(k)
			if v.Traits != nil {
				if v.Traits.Has("smithy.api#httpPayload") {
					if inputPayload {
						return fmt.Errorf("More than one @httpPayload specified in the input for operation %s", fullName)
					}
					inputPayload = true
					isPayload = true
				} else if v.Traits.Has("smithy.api#httpHeader") {
					//check header value
					isHeader = true
				} else if v.Traits.Has("smithy.api#httpLabel") {
					//check that label is present in path template
					isLabel = true
				} else if v.Traits.Has("smithy.api#httpQuery") {
					isQuery = true
				}
				if !isPayload && !isHeader && !isQuery && !isLabel {
					return fmt.Errorf("An input with no HTTP binding is present in operation %s: %s", fullName, k)
				}
			} else {
				return fmt.Errorf("An input with no HTTP binding is present in operation %s: %s", fullName, k)
			}
		}
	}
	if inputPayload != expectInputPayload {
		if inputPayload {
			return fmt.Errorf("HTTP operation '%s' with method %s expects no input payload, but one was specified", fullName, method)
		} else {
			return fmt.Errorf("HTTP operation '%s' with method %s expects an input payload, but none is specified", fullName, method)
		}
	}
	status := httpTrait.GetInt("code")
	expectOutputPayload := status != 204 && status != 304
	outputPayload := false
	if shape.Output != nil {
		outShape := ast.GetShape(shape.Output.Target)
		if outShape == nil {
			return fmt.Errorf("Undefined shape: %s\n", shape.Output.Target)
		}
		for _, k := range outShape.Members.Keys() {
			v := outShape.Members.Get(k)
			if v.Traits != nil {
				if v.Traits.Has("smithy.api#httpPayload") {
					if outputPayload {
						return fmt.Errorf("More than one @httpPayload specified in output for operation %s", fullName)
					}
					outputPayload = true
				} else if v.Traits.Has("smithy.api#httpResponseCode") {
					//
				} else if !v.Traits.Has("smithy.api#httpHeader") {
					return fmt.Errorf("An output with no HTTP binding is present in operation %s: %s", fullName, k)
				}
			} else {
				return fmt.Errorf("An output with no HTTP binding is present in operation %s: %s", fullName, k)
			}
		}
	}
	if outputPayload != expectOutputPayload {
		if outputPayload {
			return fmt.Errorf("HTTP operation '%s' with code %d expects no output payload, but one was specified", fullName, status)
		} else {
			return fmt.Errorf("HTTP operation '%s' with code %d expects an output payload, but none is specified", fullName, status)
		}
	}
	return nil
}

type SadlWriter struct {
	buf       bytes.Buffer
	writer    *bufio.Writer
	namespace string
	name      string
	ast       *AST
	config    *data.Object
}

func (gen *SadlGenerator) ToSadl(ns string, ast *AST) string {
	w := &SadlWriter{
		namespace: ns,
		ast:       ast,
		config:    gen.Config,
	}
	emitted := make(map[string]bool, 0)

	w.Begin()
	w.Emit("/* Generated from smithy source */\n")
	if ns != "" {
		w.Emit("\nnamespace %s\n", ns)
	}
	if ast.RequiresDocumentType() {
		w.Emit("\ntype Document Struct //SADL has no built-in Document type\n")
	}
	w.Emit("\n")

	for _, nsk := range ast.Shapes.Keys() {
		lst := strings.Split(nsk, "#")
		shape := ast.GetShape(nsk)
		k := lst[1]
		if shape.Type == "operation" {
			w.EmitShape(k, shape)
			emitted[k] = true
			if shape.Input != nil {
				it := w.shapeRefToTypeRef(shape.Input.Target)
				lst := strings.Split(it, "#")
				ki := lst[1]
				if vi := ast.GetShape(it); vi != nil {
					emitted[ki] = true
				}
			}
			if shape.Output != nil {
				ot := w.shapeRefToTypeRef(shape.Output.Target)
				lst := strings.Split(ot, "#")
				ko := lst[1]
				if vo := ast.GetShape(ot); vo != nil {
					emitted[ko] = true
				}
			}
		}
	}
	for _, nsk := range ast.Shapes.Keys() {
		lst := strings.Split(nsk, "#")
		k := lst[1]
		if !emitted[k] {
			w.EmitShape(k, ast.GetShape(nsk))
		}
	}
	for _, nsk := range ast.Shapes.Keys() {
		shape := ast.GetShape(nsk)
		if shape.Type == "operation" {
			if d := shape.Traits.GetArray("smithy.api#examples"); d != nil {
				for _, ex := range d {
					w.EmitExample(shape, data.AsObject(ex))
				}
			}
		}
	}
	return w.End()
}

func (w *SadlWriter) Begin() {
	w.buf.Reset()
	w.writer = bufio.NewWriter(&w.buf)
}

func (w *SadlWriter) Emit(format string, args ...interface{}) {
	w.writer.WriteString(fmt.Sprintf(format, args...))
}

func (w *SadlWriter) EmitShape(name string, shape *Shape) {
	s := strings.ToLower(shape.Type)
	if s == "service" {
		return
	}
	w.Emit("\n")
	opts := w.traitsAsAnnotations(shape.Traits)
	switch s {
	case "boolean":
		w.EmitBooleanShape(name, shape)
	case "byte":
		w.EmitNumericShape("Int8", name, shape)
	case "short":
		w.EmitNumericShape("Int16", name, shape)
	case "integer":
		w.EmitNumericShape("Int32", name, shape)
	case "long":
		w.EmitNumericShape("Int64", name, shape)
	case "float":
		w.EmitNumericShape("Float32", name, shape)
	case "double":
		w.EmitNumericShape("Float64", name, shape)
	case "biginteger":
	case "bigdecimal":
		w.EmitNumericShape("Decimal", name, shape)
	case "blob":
		w.EmitBlobShape(name, shape)
	case "string":
		w.EmitStringShape(name, shape)
	case "timestamp":
		w.EmitTimestampShape(name, shape)
	case "list":
		w.EmitCollectionShape(shape.Type, name, shape)
	case "map":
		w.EmitMapShape(name, shape)
	case "structure":
		w.EmitStructureShape(name, shape, opts)
	case "union":
		w.EmitUnionShape(name, shape)
	case "enum":
		w.EmitEnumShape(name, shape)
	case "resource":
		//no equivalent in SADL at the moment
	case "operation":
		w.EmitOperationShape(name, shape, opts)
	default:
		panic("fix: shape " + name + " of type " + data.Pretty(shape))
	}
}

func (w *SadlWriter) EmitShapeComment(shape *Shape) {
	comment := shape.Traits.GetString("smithy.api#documentation")
	if comment != "" {
		w.Emit(FormatComment("", "// ", comment, 100, true))
	}
}

func (w *SadlWriter) EmitEnumShape(name string, shape *Shape) {
	w.EmitShapeComment(shape)
	w.Emit("type %s Enum {\n", name)
	for _, k := range shape.Members.Keys() {
		//v := shape.Members.Get(k)
		//ev := v.Traits.GetString("smithy.api#enumValue")
		w.Emit("%s%s\n", IndentAmount, k)
	}
	w.Emit("}\n")
}

func (w *SadlWriter) EmitBooleanShape(name string, shape *Shape) {
	opt := ""
	w.EmitShapeComment(shape)
	w.Emit("type " + name + " Boolean" + opt + "\n")
}

func (w *SadlWriter) EmitNumericShape(shapeName, name string, shape *Shape) {
	w.EmitShapeComment(shape)
	var opts []string
	r := shape.Traits.GetObject("smithy.api#range")
	if r != nil {
		if r.Has("min") {
			opts = append(opts, fmt.Sprintf("min=%v", r.GetInt("min")))
		}
		if r.Has("max") {
			opts = append(opts, fmt.Sprintf("max=%v", r.GetInt("max")))
		}
	}
	sopts := w.annotationString(opts)
	w.Emit("type %s %s%s\n", name, shapeName, sopts)
}

func (w *SadlWriter) EmitStringShape(name string, shape *Shape) {
	w.EmitShapeComment(shape)
	var opts []string
	pat := shape.Traits.GetString("smithy.api#pattern")
	if pat != "" {
		opts = append(opts, fmt.Sprintf("pattern=%q", pat))
	}
	sopts := w.annotationString(opts)
	w.Emit("type %s String%s\n", name, sopts)
}

func (w *SadlWriter) EmitTimestampShape(name string, shape *Shape) {
	w.EmitShapeComment(shape)
	w.Emit("type %s Timestamp\n", name)
}

func (w *SadlWriter) EmitBlobShape(name string, shape *Shape) {
	w.EmitShapeComment(shape)
	opts := "" //fixme
	w.Emit("type %s Blob%s\n", name, opts)
}

func (w *SadlWriter) EmitCollectionShape(shapeName, name string, shape *Shape) {
	w.EmitShapeComment(shape)
	r := shape.Traits.GetObject("smithy.api#length")
	var opts []string
	if r != nil {
		if r.Has("min") {
			opts = append(opts, fmt.Sprintf("minsize=%v", r.GetInt("min")))
		}
		if r.Has("max") {
			opts = append(opts, fmt.Sprintf("maxsize=%v", r.GetInt("max")))
		}
	}
	sopts := w.annotationString(opts)
	//	w.EmitTraits(shape.Traits, "")
	clarifier := ""
	if shapeName != "list" {
		clarifier = " // " + shapeName
	}
	w.Emit("type %s List<%s>%s%s\n", name, w.stripNamespace(shape.Member.Target), sopts, clarifier)
}

func (w *SadlWriter) EmitMapShape(name string, shape *Shape) {
	w.EmitShapeComment(shape)
	//	w.EmitTraits(shape.Traits, "")
	w.Emit("type %s Map<%s,%s>\n", name, w.stripNamespace(shape.Key.Target), w.stripNamespace(shape.Value.Target))
}

func (w *SadlWriter) EmitStructureShape(name string, shape *Shape, opts []string) {
	sopts := w.annotationString(opts)
	w.EmitShapeComment(shape)
	w.Emit("type %s Struct%s {\n", name, sopts)
	for _, k := range shape.Members.Keys() {
		v := shape.Members.Get(k)
		tref := w.stripNamespace(w.shapeRefToTypeRef(v.Target))
		sopts := w.traitsAsAnnotationString(v.Traits)
		w.Emit("%s%s %s%s\n", IndentAmount, k, tref, sopts)
	}
	w.Emit("}\n")
}

func (w *SadlWriter) EmitUnionShape(name string, shape *Shape) {
	w.EmitShapeComment(shape)
	opt := ""
	w.Emit("type " + name + " Union" + opt + " {\n")
	for _, k := range shape.Members.Keys() {
		v := shape.Members.Get(k)
		//		w.EmitTraits(v.Traits, IndentAmount)
		tref := w.stripNamespace(w.shapeRefToTypeRef(v.Target))
		sopts := w.traitsAsAnnotationString(v.Traits)
		w.Emit("%s%s %s%s\n", IndentAmount, k, tref, sopts)
	}
	w.Emit("}\n")
}

func (w *SadlWriter) EmitOperationShape(name string, shape *Shape, opts []string) {
	httpTrait := shape.Traits.GetObject("smithy.api#http")
	if httpTrait == nil {
		return
	}
	w.EmitShapeComment(shape)
	method := httpTrait.GetString("method")
	path := httpTrait.GetString("uri")
	expected := httpTrait.GetInt("code")
	var inType string
	if shape.Input != nil {
		inType = w.shapeRefToTypeRef(shape.Input.Target)
	}
	var outType string
	if shape.Output != nil {
		outType = w.shapeRefToTypeRef(shape.Output.Target)
	}

	opts = append(opts, fmt.Sprintf("operation=%s", Uncapitalize(name)))
	sopts := "(" + strings.Join(opts, ", ") + ")"
	queryParams := ""
	var inShape *Shape
	inputIsPayload := method == "PUT" || method == "POST" || method == "PATCH"
	if inType != "" {
		inShape = w.ast.GetShape(inType)
		if inShape == nil {
			panic("cannot find shape def for: " + inType)
		}
		for _, k := range inShape.Members.Keys() {
			v := inShape.Members.Get(k)
			if v.Traits != nil {
				if v.Traits.Has("smithy.api#httpPayload") {
					inputIsPayload = false
					break
				}
				s := v.Traits.GetString("smithy.api#httpQuery")
				if s != "" {
					p := s + "={" + k + "}"
					if queryParams == "" {
						queryParams = "?" + p
					} else {
						queryParams = queryParams + "&" + p
					}
				}
			}
		}
	}
	w.Emit("http %s %q %s {\n", method, path+queryParams, sopts)
	if inShape != nil {
		if inputIsPayload {
			k := "body"
			tref := w.stripNamespace(inType)
			w.Emit("\t%s %s (required)\n", k, tref)
		} else {
			for _, k := range inShape.Members.Keys() {
				v := inShape.Members.Get(k)
				var mopts []string
				if v.Traits.Has("smithy.api#httpPayload") {
					mopts = append(mopts, "required")
				} else {
					s := v.Traits.GetString("smithy.api#httpQuery")
					if s != "" {
						//smithy has no "default" option
					} else {
						if v.Traits.Has("smithy.api#httpLabel") {
							mopts = append(mopts, "required")
						} else {
							s = v.Traits.GetString("smithy.api#httpHeader")
							if s != "" {
								mopts = append(mopts, fmt.Sprintf("header=%q", s))
							}
						}
					}
				}
				sopts := ""
				if len(mopts) > 0 {
					sopts = " (" + strings.Join(mopts, ",") + ")"
				}
				tref := w.stripNamespace(w.shapeRefToTypeRef(v.Target))
				w.Emit("\t%s %s%s\n", k, tref, sopts)
			}
		}
		w.Emit("\n")
	}
	var outShape *Shape
	var mopts []string
	if outType != "" {
		outShape = w.ast.GetShape(outType)
		w.Emit("\texpect %d {\n", expected)
		for _, k := range outShape.Members.Keys() {
			v := outShape.Members.Get(k)
			if v.Traits.Has("smithy.api#httpPayload") {
			} else {
				s := v.Traits.GetString("smithy.api#httpHeader")
				if s != "" {
					mopts = append(mopts, fmt.Sprintf("header=%q", s))
				}
			}
			sopts := ""
			if len(mopts) > 0 {
				sopts = " (" + strings.Join(mopts, ", ") + ")"
			}
			tref := w.stripNamespace(w.shapeRefToTypeRef(v.Target))
			w.Emit("\t\t%s %s%s\n", k, tref, sopts)
		}
		w.Emit("\t}\n")
	} else {
		w.Emit("\texpect %d\n", expected) //no content
	}
	//except: we have to iterate through the "errors" of the operation, and check each one for httpError
	//Note that there is in that case not much opportunity to do headers.
	if len(shape.Errors) > 0 {
		for _, errType := range shape.Errors {
			errShape := w.ast.GetShape(errType.Target)
			if errShape == nil {
				fmt.Println(data.Pretty(errType))
				panic("whoops, no error?")
			}
			errCode := errShape.Traits.GetInt("smithy.api#httpError")
			if errCode != 0 {
				w.Emit("\texcept %d %s\n", errCode, w.stripNamespace(errType.Target))
			}
		}
	}
	w.Emit("}\n")
}

func (w *SadlWriter) EmitExample(shape *Shape, obj *data.Object) {
	opName := obj.GetString("title")
	if obj.Has("input") {
		reqType := w.stripNamespace(shape.Input.Target)
		w.Emit("\nexample %s (name=%s) ", reqType, opName)
		w.Emit(data.Pretty(obj.GetObject("input")))
	}
	if obj.Has("error") {
		er := obj.GetObject("error")
		respType := w.stripNamespace(er.GetString("shapeId"))
		w.Emit("\nexample %s (name=%s) ", respType, opName)
		w.Emit(data.Pretty(er.GetObject("error")))
	} else {
		respType := w.stripNamespace(shape.Output.Target)
		w.Emit("\nexample %s (name=%s) ", respType, opName)
		w.Emit(data.Pretty(obj.GetObject("output")))
	}
}

func (w *SadlWriter) End() string {
	w.writer.Flush()
	return w.buf.String()
}

/*
   func (gen *SadlGenerator) serviceName(model *Model, ns string) (string, *Shape) {
	for _, nsk := range model.ast.Shapes.Keys() {
		shape := model.ast.GetShape(nsk)
		shapeAbsName := strings.Split(nsk, "#")
		shapeNs := shapeAbsName[0]
		shapeName := shapeAbsName[1]
		if shapeNs == ns {
			if shape.Type == "service" {
				return shapeName, shape
			}
		}
	}
	return "", nil
}
*/

func (w *SadlWriter) stripNamespace(id string) string {
	//fixme: just totally ignore it for now
	n := strings.Index(id, "#")
	if n < 0 {
		return id
	} else {
		return id[n+1:]
	}
	/*
		match := w.namespace + "#"
		if strings.HasPrefix(id, match) {
			return id[len(match):]
		}
		if strings.HasPrefix(id, "smithy.api") {
			n := strings.Index(id, "#")
			if n >= 0 {
				return id[n+1:]
			}
		}
		return id
	*/
}

func (w *SadlWriter) formatBlockComment(indent string, comment string) {
}

func (w *SadlWriter) shapeRefToTypeRef(shapeRef string) string {
	typeRef := shapeRef
	switch typeRef {
	case "smithy.api#Blob", "Blob":
		return "Bytes"
	case "smithy.api#Boolean", "Boolean":
		return "Bool"
	case "smithy.api#String", "String":
		return "String"
	case "smithy.api#Byte", "Byte":
		return "Int8"
	case "smithy.api#Short", "Short":
		return "Int16"
	case "smithy.api#Integer", "Integer":
		return "Int32"
	case "smithy.api#Long", "Long":
		return "Int64"
	case "smithy.api#Float", "Float":
		return "Float32"
	case "smithy.api#Double", "Double":
		return "Float64"
	case "smithy.api#BigInteger", "BigInteger":
		return "Decimal" //lossy!
	case "smithy.api#BigDecimal", "BigDecimal":
		return "Decimal"
	case "smithy.api#Timestamp", "Timestamp":
		return "Timestamp"
	case "smithy.api#Document", "Document":
		return "Document" //to do: a new primitive type for this. For now, a naked Struct works
	default:
		//		ltype := model.ensureLocalNamespace(typeRef)
		//		if ltype == "" {
		//			panic("external namespace type refr not supported: " + typeRef)
		//		}
		//implement "use" correctly to handle this.
		//typeRef = ltype
	}
	return typeRef
}

func withAnnotation(annos map[string]string, key string, value string) map[string]string {
	if value != "" {
		if annos == nil {
			annos = make(map[string]string, 0)
		}
		annos[key] = value
	}
	return annos
}

func (w *SadlWriter) annotationString(opts []string) string {
	if len(opts) == 0 {
		return ""
	}
	return fmt.Sprintf(" (%s)", strings.Join(opts, ", "))
}

func (w *SadlWriter) traitsAsAnnotationString(traits *data.Object) string {
	return w.annotationString(w.traitsAsAnnotations(traits))
}

func (w *SadlWriter) traitsAsAnnotations(traits *data.Object) []string {
	var opts []string
	if traits != nil {
		for _, k := range traits.Keys() {
			v := traits.Get(k)
			switch k {
			case "smithy.api#required":
				opts = append(opts, "required")
			case "smithy.api#deprecated":
				if w.config.GetBool("annotate") {
					//				dv := data.AsMap(v)
					dv := data.AsObject(v)
					msg := dv.GetString("message")
					opts = append(opts, fmt.Sprintf("x_deprecated=%q", msg))
				}
				/*
					case "smithy.api#paginated":
							dv := sadl.AsMap(v)
							inputToken := sadl.AsString(dv["inputToken"])
							outputToken := sadl.AsString(dv["outputToken"])
							pageSize := sadl.AsString(dv["pageSize"])
							items := sadl.AsString(dv["items"])
							s := fmt.Sprintf("inputToken=%s,outputToken=%s,pageSize=%s,items=%s", inputToken, outputToken, p\
								ageSize, items)
							annos = WithAnnotation(annos, "x_paginated", s)
				*/
			case "smithy.api#timestampFormat":
				if w.config.GetBool("annotate") {
					opts = append(opts, fmt.Sprintf("x_timestampFormat=%q", v))
				}
			case "smithy.api#tags":
				if w.config.GetBool("annotate") {
					opts = append(opts, fmt.Sprintf("x_tags=%q", strings.Join(data.AsStringArray(v), ",")))
				}
			case "smithy.api#error":
				if w.config.GetBool("annotate") {
					opts = append(opts, "x_error")
				}
			case "smithy.api#httpError":
				if w.config.GetBool("annotate") {
					opts = append(opts, fmt.Sprintf("x_httpError=\"%v\"", v))
				}
			}
		}
	}
	return opts
}
