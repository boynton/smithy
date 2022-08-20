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

const IndentAmount = "    "

//ASTs don't have a preferred namespace, but IDL files do. When going back to IDL, getting the preferred namespace is desirable.
//The algorithm here is to prefer the first service's namespace, if present, or the first non-smithy, non-aws namespace encountered.
func (ast *AST) NamespaceAndServiceVersion() (string, string, string) {
	var namespace, name, version string
	for _, k := range ast.Shapes.Keys() {
		v := ast.GetShape(k)
		if strings.HasPrefix(k, "smithy.") || strings.HasPrefix(k, "aws.") {
			continue
		}
		i := strings.Index(k, "#")
		if i >= 0 {
			namespace = k[:i]
		}
		if v.Type == "service" {
			version = v.Version
			name = k[i+1:]
			break
		}
	}
	return namespace, name, version
}

//
// Generate Smithy IDL to describe the Smithy model for a specified namespace
//
func (ast *AST) IDL(ns string) string {
	w := &IdlWriter{
		ast:       ast,
		namespace: ns,
		version:   ast.AssemblyVersion(),
	}

	w.Begin()
	w.Emit("$version: \"%d\"\n", w.version)
	emitted := make(map[string]bool, 0)

	if ast.Metadata.Length() > 0 {
		w.Emit("\n")
		for _, k := range ast.Metadata.Keys() {
			v := ast.Metadata.Get(k)
			w.Emit("metadata %s = %s", k, data.Pretty(v))
		}
	}
	w.Emit("\nnamespace %s\n", ns)

	imports := ast.ExternalRefs(ns)
	if len(imports) > 0 {
		w.Emit("\n")
		for _, im := range imports {
			w.Emit("use %s\n", im)
		}
	}

	for _, nsk := range ast.Shapes.Keys() {
		shape := ast.GetShape(nsk)
		shapeAbsName := strings.Split(nsk, "#")
		shapeNs := shapeAbsName[0]
		shapeName := shapeAbsName[1]
		if shapeNs == ns {
			if shape.Type == "service" {
				w.Emit("\n")
				w.EmitServiceShape(shapeName, shape)
				break
			}
		}
	}
	for _, nsk := range ast.Shapes.Keys() {
		lst := strings.Split(nsk, "#")
		if lst[0] == ns {
			shape := ast.GetShape(nsk)
			k := lst[1]
			if shape.Type == "operation" {
				w.Emit("\n")
				w.EmitOperationShape(k, shape, emitted)
			}
		}
	}
	for _, nsk := range ast.Shapes.Keys() {
		lst := strings.Split(nsk, "#")
		k := lst[1]
		if lst[0] == ns {
			if !emitted[k] {
				w.EmitShape(k, ast.GetShape(nsk))
			}
		}
	}
	for _, nsk := range ast.Shapes.Keys() {
		shape := ast.GetShape(nsk)
		if shape.Type == "operation" {
			lst := strings.Split(nsk, "#")
			if lst[0] == ns {
				if d := shape.Traits.Get("smithy.api#examples"); d != nil {
					switch v := d.(type) {
					case []map[string]interface{}:
						w.EmitExamplesTrait(nsk, v)
					}
				}
			}
		}
	}
	return w.End()
}

func (ast *AST) ExternalRefs(ns string) []string {
	match := ns + "#"
	if ns == "" {
		match = ""
	}
	refs := make(map[string]bool, 0)
	for _, k := range ast.Shapes.Keys() {
		lst := strings.Split(k, "#")
		if ns == "" || lst[0] == ns {
			v := ast.GetShape(k)
			ast.noteExternalRefs(match, k, v, refs)
		}
	}
	var res []string
	for k, _ := range refs {
		res = append(res, k)
	}
	return res
}

func (ast *AST) noteExternalTraitRefs(match string, traits *data.Object, refs map[string]bool) {
	if traits != nil {
		for _, tk := range traits.Keys() {
			if !strings.HasPrefix(tk, "smithy.api#") && (match != "" && !strings.HasPrefix(tk, match)) {
				refs[tk] = true
			}
		}
	}
}

func (ast *AST) noteExternalRefs(match string, name string, shape *Shape, refs map[string]bool) {
	if name == "smithy.api#Document" {
		//force an alias to this to get emitted.
	} else if strings.HasPrefix(name, "smithy.api#") {
		return
	}
	if _, ok := refs[name]; ok {
		return
	}
	if match == "" || !strings.HasPrefix(name, match) {
		refs[name] = true
		if shape != nil {
			ast.noteExternalTraitRefs(match, shape.Traits, refs)
			switch shape.Type {
			case "map":
				ast.noteExternalRefs(match, shape.Key.Target, ast.GetShape(shape.Key.Target), refs)
				ast.noteExternalTraitRefs(match, shape.Key.Traits, refs)
				ast.noteExternalRefs(match, shape.Value.Target, ast.GetShape(shape.Value.Target), refs)
				ast.noteExternalTraitRefs(match, shape.Value.Traits, refs)
			case "list", "set":
				ast.noteExternalRefs(match, shape.Member.Target, ast.GetShape(shape.Member.Target), refs)
				ast.noteExternalTraitRefs(match, shape.Member.Traits, refs)
			case "structure", "union":
				if shape.Members != nil {
					for _, k := range shape.Members.Keys() {
						member := shape.Members.Get(k)
						ast.noteExternalRefs(match, member.Target, ast.GetShape(member.Target), refs)
						ast.noteExternalTraitRefs(match, member.Traits, refs)
					}
				}
			}
		}
	}
}

type IdlWriter struct {
	buf       bytes.Buffer
	writer    *bufio.Writer
	namespace string
	name      string
	version   int
	ast       *AST
}

func (w *IdlWriter) Begin() {
	w.buf.Reset()
	w.writer = bufio.NewWriter(&w.buf)
}

func (w *IdlWriter) stripNamespace(id string) string {
	n := strings.Index(id, "#")
	if n < 0 {
		return id
	}
	return id[n+1:]
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

func (w *IdlWriter) Emit(format string, args ...interface{}) {
	w.writer.WriteString(fmt.Sprintf(format, args...))
}

func (w *IdlWriter) EmitShape(name string, shape *Shape) {
	s := strings.ToLower(shape.Type)
	w.Emit("\n")
	switch s {
	case "boolean":
		w.EmitBooleanShape(name, shape)
	case "byte", "short", "integer", "long", "float", "double", "bigInteger", "bigdecimal":
		w.EmitNumericShape(shape.Type, name, shape)
	case "blob":
		w.EmitBlobShape(name, shape)
	case "string":
		w.EmitStringShape(name, shape)
	case "timestamp":
		w.EmitTimestampShape(name, shape)
	case "list", "set":
		w.EmitCollectionShape(shape.Type, name, shape)
	case "map":
		w.EmitMapShape(name, shape)
	case "structure":
		w.EmitStructureShape(name, shape)
	case "union":
		w.EmitUnionShape(name, shape)
	case "enum":
		w.EmitEnumShape(name, shape)
	case "resource":
		w.EmitResourceShape(name, shape)
	case "operation", "service":
		// already emitted
		// w.EmitOperationShape(name, shape, emitted)
	default:
		panic("fix: shape " + name + " of type " + data.Pretty(shape))
	}
}

func (w *IdlWriter) EmitDocumentation(doc, indent string) {
	if doc != "" {
		s := FormatComment(indent, "/// ", doc, 100, false)
		w.Emit(s)
		//		w.Emit("%s@documentation(%q)\n", indent, doc)
	}
}

func (w *IdlWriter) EmitBooleanTrait(b bool, tname, indent string) {
	if b {
		w.Emit("%s@%s\n", indent, tname)
	}
}

func (w *IdlWriter) EmitStringTrait(v, tname, indent string) {
	if v != "" {
		if v == "-" { //hack
			w.Emit("%s@%s\n", indent, tname)
		} else {
			w.Emit("%s@%s(%q)\n", indent, tname, v)
		}
	}
}

func (w *IdlWriter) EmitLengthTrait(v interface{}, indent string) {
	l := data.AsMap(v)
	min := data.Get(l, "min")
	max := data.Get(l, "max")
	if min != nil && max != nil {
		w.Emit("@length(min: %d, max: %d)\n", data.AsInt(min), data.AsInt(max))
	} else if max != nil {
		w.Emit("@length(max: %d)\n", data.AsInt(max))
	} else if min != nil {
		w.Emit("@length(min: %d)\n", data.AsInt(min))
	}
}

func (w *IdlWriter) EmitRangeTrait(v interface{}, indent string) {
	l := data.AsMap(v)
	min := data.Get(l, "min")
	max := data.Get(l, "max")
	if min != nil && max != nil {
		w.Emit("@range(min: %v, max: %v)\n", data.AsDecimal(min), data.AsDecimal(max))
	} else if max != nil {
		w.Emit("@range(max: %v)\n", data.AsDecimal(max))
	} else if min != nil {
		w.Emit("@range(min: %v)\n", data.AsDecimal(min))
	}
}

func (w *IdlWriter) EmitEnumTrait(v interface{}, indent string) {
	en := v.([]interface{})
	if len(en) > 0 {
		s := data.Pretty(en)
		slen := len(s)
		if slen > 0 && s[slen-1] == '\n' {
			s = s[:slen-1]
		}
		w.Emit("@enum(%s)\n", s)
	}
}

func (w *IdlWriter) EmitTraitTrait(v interface{}) {
	l := data.AsMap(v)
	if l != nil {
		var lst []string
		selector := data.GetString(l, "selector")
		if selector != "" {
			lst = append(lst, fmt.Sprintf("selector: %q", selector))
		}
		conflicts := data.GetStringArray(l, "conflicts")
		if conflicts != nil {
			s := "["
			for _, e := range conflicts {
				if s != "[" {
					s = s + ", "
				}
				s = s + e
			}
			s = s + "]"
			lst = append(lst, fmt.Sprintf("conflicts: %s", s))
		}
		structurallyExclusive := data.GetString(l, "structurallyExclusive")
		if structurallyExclusive != "" {
			lst = append(lst, fmt.Sprintf("selector: %q", structurallyExclusive))
		}
		if len(lst) > 0 {
			w.Emit("@trait(%s)\n", strings.Join(lst, ", "))
			return
		}
	}
	w.Emit("@trait\n")
}

func (w *IdlWriter) EmitTagsTrait(v interface{}, indent string) {
	if sa, ok := v.([]string); ok {
		w.Emit("@tags(%v)\n", listOfStrings("", "%q", sa))
	}
}

func (w *IdlWriter) EmitDeprecatedTrait(v interface{}, indent string) {
	dep := data.AsObject(v)
	if dep != nil {
		s := indent + "@deprecated"
		hasMessage := false
		if dep.Has("message") {
			s = s + fmt.Sprintf("(message: %q", dep.GetString("message"))
			hasMessage = true
		}
		if dep.Has("since") {
			if hasMessage {
				s = s + fmt.Sprintf(", since: %q)", dep.GetString("since"))
			} else {
				s = s + fmt.Sprintf("(since: %q)", dep.GetString("since"))
			}
		} else {
			s = s + ")"
		}
		w.Emit(s + "\n")
	}
}

func (w *IdlWriter) EmitHttpTrait(rv interface{}, indent string) {
	var method, uri string
	code := 0
	switch v := rv.(type) {
	case map[string]interface{}:
		method = data.GetString(v, "method")
		uri = data.GetString(v, "uri")
		code = data.GetInt(v, "code")
	case *data.Object:
		method = data.AsString(v.Get("method"))
		uri = data.AsString(v.Get("uri"))
		code = data.AsInt(v.Get("code"))
	default:
		panic("What?!")
	}
	s := fmt.Sprintf("method: %q, uri: %q", method, uri)
	if code != 0 {
		s = s + fmt.Sprintf(", code: %d", code)
	}
	w.Emit("@http(%s)\n", s)
}

func (w *IdlWriter) EmitHttpErrorTrait(rv interface{}, indent string) {
	var status int
	switch v := rv.(type) {
	case int32:
		status = int(v)
	default:
		//		fmt.Printf("http error arg, expected an int32, found %s with type %s\n", rv, Kind(rv))
	}
	if status != 0 {
		w.Emit("@httpError(%d)\n", status)
	}
}

func (w *IdlWriter) EmitSimpleShape(shapeName, name string, shape *Shape) {
	w.Emit("%s %s%s\n", shapeName, name, w.withMixins(shape.Mixins))
}

func (w *IdlWriter) EmitBooleanShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.EmitSimpleShape("boolean", name, shape)
}

func (w *IdlWriter) EmitNumericShape(shapeName, name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.EmitSimpleShape(shapeName, name, shape)
}

func (w *IdlWriter) EmitStringShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.EmitSimpleShape(shape.Type, name, shape)
}

func (w *IdlWriter) withMixins(mixins []*ShapeRef) string {
	if len(mixins) == 0 {
		return ""
	}
	var mixinNames []string
	for _, ref := range mixins {
		mixinNames = append(mixinNames, w.stripNamespace(ref.Target))
	}
	return fmt.Sprintf(" with [%s]", strings.Join(mixinNames, ", "))
}

func (w *IdlWriter) EmitTimestampShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("timestamp %s%s\n", name, w.withMixins(shape.Mixins))
}

func (w *IdlWriter) EmitBlobShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("blob %s%s\n", name, w.withMixins(shape.Mixins))
}

func (w *IdlWriter) EmitCollectionShape(shapeName, name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("%s %s%s {\n", shapeName, name, w.withMixins(shape.Mixins))
	w.Emit("    member: %s\n", w.stripNamespace(shape.Member.Target))
	w.Emit("}\n")
}

func (w *IdlWriter) EmitMapShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("map %s%s {\n    key: %s,\n    value: %s\n}\n", name, w.withMixins(shape.Mixins), w.stripNamespace(shape.Key.Target), w.stripNamespace(shape.Value.Target))
}

func (w *IdlWriter) EmitUnionShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("union %s%s {\n", name, w.withMixins(shape.Mixins))
	count := shape.Members.Length()
	for _, fname := range shape.Members.Keys() {
		mem := shape.Members.Get(fname)
		w.EmitTraits(mem.Traits, IndentAmount)
		w.Emit("%s%s: %s", IndentAmount, fname, w.stripNamespace(mem.Target))
		count--
		if count > 0 {
			w.Emit(",\n")
		} else {
			w.Emit("\n")
		}
	}
	w.Emit("}\n")
}

func (w *IdlWriter) EmitEnumShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("enum %s%s {\n", name, w.withMixins(shape.Mixins))
	count := shape.Members.Length()
	for _, fname := range shape.Members.Keys() {
		mem := shape.Members.Get(fname)
		sval := fname
		eqval := ""
		if val := mem.Traits.Get("smithy.api#enumValue"); val != nil {
			sval = data.AsString(val)
			fmt.Println("sval, fname:", sval, fname)
			if sval != fname {
				eqval = fmt.Sprintf(" = %q", sval)
			}
		}
		w.EmitTraits(mem.Traits, IndentAmount)
		w.Emit("%s%s%s", IndentAmount, fname, eqval)
		count--
		if count > 0 {
			w.Emit(",\n")
		} else {
			w.Emit("\n")
		}
	}
	w.Emit("}\n")
}

func (w *IdlWriter) EmitTraits(traits *data.Object, indent string) {
	//note: @documentation is an alternate for ("///"+comment), but then must be before other traits.
	if traits == nil {
		return
	}
	for _, k := range traits.Keys() {
		v := traits.Get(k)
		switch k {
		case "smithy.api#documentation":
			w.EmitDocumentation(data.AsString(v), indent)
		}
	}
	for _, k := range traits.Keys() {
		v := traits.Get(k)
		switch k {
		case "smithy.api#documentation", "smithy.api#examples", "smithy.api#enumValue":
			//do nothing, handled elsewhere
		case "smithy.api#sensitive", "smithy.api#required", "smithy.api#readonly", "smithy.api#idempotent":
			w.EmitBooleanTrait(data.AsBool(v), w.stripNamespace(k), indent)
		case "smithy.api#httpLabel", "smithy.api#httpPayload":
			w.EmitBooleanTrait(data.AsBool(v), w.stripNamespace(k), indent)
		case "smithy.api#httpQuery", "smithy.api#httpHeader", "smithy.api#timestampFormat":
			w.EmitStringTrait(data.AsString(v), w.stripNamespace(k), indent)
		case "smithy.api#deprecated":
			w.EmitDeprecatedTrait(v, indent)
		case "smithy.api#http":
			w.EmitHttpTrait(v, indent)
		case "smithy.api#httpError":
			w.EmitHttpErrorTrait(v, indent)
		case "smithy.api#length":
			w.EmitLengthTrait(v, indent)
		case "smithy.api#range":
			w.EmitRangeTrait(v, indent)
		case "smithy.api#enum":
			w.EmitEnumTrait(v, indent)
		case "smithy.api#tags":
			w.EmitTagsTrait(v, indent)
		case "smithy.api#pattern", "smithy.api#error":
			w.EmitStringTrait(data.AsString(v), w.stripNamespace(k), indent)
		case "aws.protocols#restJson1":
			w.Emit("%s@%s\n", indent, k) //FIXME for the non-default attributes
		case "smithy.api#paginated":
			w.EmitPaginatedTrait(v)
		case "smithy.api#trait":
			w.EmitTraitTrait(v)
		default:
			w.EmitCustomTrait(k, v, indent)
		}
	}
}

func (w *IdlWriter) EmitCustomTrait(k string, v interface{}, indent string) {
	args := ""
	if m, ok := v.(*data.Object); ok {
		if m.Length() > 0 {
			var lst []string
			for _, ak := range m.Keys() {
				av := m.Get(ak)
				lst = append(lst, fmt.Sprintf("%s: %s", ak, data.Json(av)))
			}
			args = "(\n    " + strings.Join(lst, ",\n    ") + ")"
		}
	}
	w.Emit("%s@%s%s\n", indent, w.stripNamespace(k), args)
}

func (w *IdlWriter) EmitPaginatedTrait(d interface{}) {
	if m, ok := d.(map[string]interface{}); ok {
		var args []string
		for k, v := range m {
			args = append(args, fmt.Sprintf("%s: %q", k, v))
		}
		if len(args) > 0 {
			w.Emit("@paginated(" + strings.Join(args, ", ") + ")\n")
		}
	}
}

func (w *IdlWriter) EmitExamplesTrait(opname string, raw interface{}) {
	switch dat := raw.(type) {
	case []map[string]interface{}:
		target := w.stripNamespace(opname)
		formatted := data.Pretty(dat)
		if strings.HasSuffix(formatted, "\n") {
			formatted = formatted[:len(formatted)-1]
		}
		w.Emit("apply "+target+" @examples(%s)\n", formatted)
	default:
		panic("FIX ME!")
	}
}

func (w *IdlWriter) EmitStructureShape(name string, shape *Shape) {
	comma := ""
	if w.version < 2 {
		comma = ","
	}
	w.EmitTraits(shape.Traits, "")
	w.Emit("structure %s%s {\n", name, w.withMixins(shape.Mixins))
	for i, k := range shape.Members.Keys() {
		if i > 0 {
			w.Emit("\n")
		}
		v := shape.Members.Get(k)
		w.EmitTraits(v.Traits, IndentAmount)
		w.Emit("%s%s: %s%s\n", IndentAmount, k, w.stripNamespace(v.Target), comma)
	}
	w.Emit("}\n")
}

func (w *IdlWriter) listOfShapeRefs(label string, format string, lst []*ShapeRef, absolute bool) string {
	s := ""
	if len(lst) > 0 {
		s = label + ": ["
		for n, a := range lst {
			if n > 0 {
				s = s + ", "
			}
			target := a.Target
			if !absolute {
				target = w.stripNamespace(target)
			}
			s = s + fmt.Sprintf(format, target)
		}
		s = s + "]"
	}
	return s
}

func listOfStrings(label string, format string, lst []string) string {
	s := ""
	if len(lst) > 0 {
		if label != "" {
			s = label + ": "
		}
		s = s + "["
		for n, a := range lst {
			if n > 0 {
				s = s + ", "
			}
			s = s + fmt.Sprintf(format, a)
		}
		s = s + "]"
	}
	return s
}

func (w *IdlWriter) EmitServiceShape(name string, shape *Shape) {
	comma := ""
	if w.version < 2 {
		comma = ","
	}
	w.EmitTraits(shape.Traits, "")
	w.Emit("service %s%s {\n", name, w.withMixins(shape.Mixins))
	w.Emit("    version: %q%s\n", shape.Version, comma)
	if len(shape.Operations) > 0 {
		w.Emit("    %s\n", w.listOfShapeRefs("operations", "%s", shape.Operations, false))
	}
	if len(shape.Resources) > 0 {
		w.Emit("    %s\n", w.listOfShapeRefs("resources", "%s", shape.Resources, false))
	}
	w.Emit("}\n")
}

func (w *IdlWriter) EmitResourceShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("resource %s%s {\n", name, w.withMixins(shape.Mixins))
	if len(shape.Identifiers) > 0 {
		w.Emit("    identifiers: {\n")
		for k, v := range shape.Identifiers {
			w.Emit("        %s: %s,\n", k, w.stripNamespace(v.Target))
		}
		w.Emit("    }\n")
		if shape.Create != nil {
			w.Emit("    create: %v\n", w.stripNamespace(shape.Create.Target))
		}
		if shape.Put != nil {
			w.Emit("    put: %v\n", w.stripNamespace(shape.Put.Target))
		}
		if shape.Read != nil {
			w.Emit("    read: %v\n", w.stripNamespace(shape.Read.Target))
		}
		if shape.Update != nil {
			w.Emit("    update: %v\n", w.stripNamespace(shape.Update.Target))
		}
		if shape.Delete != nil {
			w.Emit("    delete: %v\n", w.stripNamespace(shape.Delete.Target))
		}
		if shape.List != nil {
			w.Emit("    list: %v\n", w.stripNamespace(shape.List.Target))
		}
		if len(shape.Operations) > 0 {
			var tmp []*ShapeRef
			for _, id := range shape.Operations {
				tmp = append(tmp, &ShapeRef{Target: w.stripNamespace(id.Target)})
			}
			w.Emit("    %s\n", w.listOfShapeRefs("operations", "%s", tmp, true))
		}
		if len(shape.CollectionOperations) > 0 {
			w.Emit("    %s\n", w.listOfShapeRefs("collectionOperations", "%s", shape.CollectionOperations, true))
		}
	}
	w.Emit("}\n")
}

func (w *IdlWriter) EmitOperationShape(name string, shape *Shape, emitted map[string]bool) {
	var inputShape, outputShape *Shape
	var inputName, outputName string
	if shape.Input != nil {
		inputName = w.stripNamespace(shape.Input.Target)
		inputShape = w.ast.GetShape(shape.Input.Target)
	}
	if shape.Output != nil {
		outputName = w.stripNamespace(shape.Output.Target)
		outputShape = w.ast.GetShape(shape.Output.Target)
	}
	w.EmitTraits(shape.Traits, "")
	w.Emit("operation %s%s {\n", name, w.withMixins(shape.Mixins))
	if w.version == 2 {
		if inputShape != nil {
			if b := inputShape.Traits.Get("smithy.api#input"); b != nil {
				inputTraits := "" //?
				inputMixins := w.withMixins(inputShape.Mixins)
				w.Emit("%sinput := %s%s{\n", IndentAmount, inputTraits, inputMixins)
				i2 := IndentAmount + IndentAmount
				for i, k := range inputShape.Members.Keys() {
					if i > 0 {
						w.Emit("\n")
					}
					v := inputShape.Members.Get(k)
					w.EmitTraits(v.Traits, i2)
					w.Emit("%s%s: %s\n", i2, k, w.stripNamespace(v.Target))
				}
				w.Emit("%s}\n", IndentAmount)
			} else {
				w.Emit("%sinput: %s,\n", IndentAmount, w.stripNamespace(inputName))
			}
		}
		if outputShape != nil { //probably should require the @output trait before inlining.
			if b := outputShape.Traits.Get("smithy.api#output"); b != nil {
				w.Emit("%soutput := {\n", IndentAmount)
				i2 := IndentAmount + IndentAmount
				for i, k := range outputShape.Members.Keys() {
					if i > 0 {
						w.Emit("\n")
					}
					v := outputShape.Members.Get(k)
					w.EmitTraits(v.Traits, i2)
					w.Emit("%s%s: %s\n", i2, k, w.stripNamespace(v.Target))
				}
				w.Emit("%s}\n", IndentAmount)
			} else {
				w.Emit("%soutput: %s,\n", IndentAmount, w.stripNamespace(outputName))
			}
		}
		if len(shape.Errors) > 0 {
			w.Emit("    %s\n", w.listOfShapeRefs("errors", "%s", shape.Errors, false))
		}
	} else {
		if shape.Input != nil {
			w.Emit("    input: %s,\n", inputName)
		}
		if shape.Output != nil {
			w.Emit("    output: %s,\n", outputName)
		}
		if len(shape.Errors) > 0 {
			w.Emit("    %s,\n", w.listOfShapeRefs("errors", "%s", shape.Errors, false))
		}
	}
	w.Emit("}\n")
	emitted[name] = true
	if inputShape != nil {
		if w.version == 1 {
			w.EmitShape(inputName, inputShape)
		}
		emitted[inputName] = true
	}
	if outputShape != nil {
		if w.version == 1 {
			w.EmitShape(outputName, outputShape)
		}
		emitted[outputName] = true
	}
}

func (w *IdlWriter) End() string {
	w.writer.Flush()
	return w.buf.String()
}
