package smithy

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

//
// Fix me: the unparser needs to unparse to a set of files, not a single file. One file for each namespace.
// When generating to stdio, these files should be concatenated, one per
const IndentAmount = "    "

//ASTs don't have a preferred namespac, but IDL files do. When going back to IDL, getting the preferred namespace is desirable
//This project's parser emits metadata for the preferred namespace, but if that is missing, we have to guess. The algorithm here
//is to prefer the first service's namespace, if present, or the first non-non-smithy, non-aws namespace encountered.
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
		namespace: ns,
	}

	w.Begin()
	w.Emit("$version: %q\n", ast.Smithy) //only if a version-specific feature is needed. Could be "1" or "1.0"
	emitted := make(map[string]bool, 0)
	if ast.Metadata.Length() > 0 {
		w.Emit("\n")
		for _, k := range ast.Metadata.Keys() {
			v := ast.Metadata.Get(k)
			w.Emit("metadata %s = %s", k, Pretty(v))
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
				w.EmitShape(k, shape)
				emitted[k] = true
				ki := k + "Input"
				if vi := ast.GetShape(ns + "#" + ki); vi != nil {
					w.EmitShape(ki, vi)
					emitted[ki] = true
				}
				ko := k + "Output"
				if vo := ast.GetShape(ns + "#" + ko); vo != nil {
					w.EmitShape(ko, vo)
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
			if d := shape.Traits.Get("smithy.api#examples"); d != nil {
				switch v := d.(type) {
				case []map[string]interface{}:
					w.EmitExamplesTrait(nsk, v)
				}
			}
		}
	}
	return w.End()
}

func (ast *AST) ExternalRefs(ns string) []string {
	match := ns + "#"
	refs := make(map[string]bool, 0)
	for _, k := range ast.Shapes.Keys() {
		v := ast.GetShape(k)
		ast.noteExternalRefs(match, k, v, refs)
	}
	var res []string
	for k, _ := range refs {
		res = append(res, k)
	}
	return res
}

func (ast *AST) noteExternalTraitRefs(match string, traits *Struct, refs map[string]bool) {
	if traits != nil {
		for _, tk := range traits.Keys() {
			if !strings.HasPrefix(tk, "smithy.api#") && !strings.HasPrefix(tk, match) {
				refs[tk] = true
			}
		}
	}
}

func (ast *AST) noteExternalRefs(match string, name string, shape *Shape, refs map[string]bool) {
	if strings.HasPrefix(name, "smithy.api#") {
		return
	}
	if _, ok := refs[name]; ok {
		return
	}
	if !strings.HasPrefix(name, match) {
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
				for _, member := range shape.Members {
					ast.noteExternalRefs(match, member.Target, ast.GetShape(member.Target), refs)
					ast.noteExternalTraitRefs(match, member.Traits, refs)
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
}

func (w *IdlWriter) Begin() {
	w.buf.Reset()
	w.writer = bufio.NewWriter(&w.buf)
}

func (w *IdlWriter) stripNamespace(id string) string {
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
}

func (w *IdlWriter) Emit(format string, args ...interface{}) {
	w.writer.WriteString(fmt.Sprintf(format, args...))
}

func (w *IdlWriter) EmitShape(name string, shape *Shape) {
	s := strings.ToLower(shape.Type)
	if s == "service" {
		return
	}
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
	case "resource":
		w.EmitResourceShape(name, shape)
	case "operation":
		w.EmitOperationShape(name, shape)
	default:
		panic("fix: shape " + name + " of type " + Pretty(shape))
	}
}

func (w *IdlWriter) EmitDocumentation(doc, indent string) {
	if doc != "" {
		s := FormatComment("", "/// ", doc, 100, false)
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
	l := AsMap(v)
	min := Get(l, "min")
	max := Get(l, "max")
	if min != nil && max != nil {
		w.Emit("@length(min: %d, max: %d)\n", AsInt(min), AsInt(max))
	} else if max != nil {
		w.Emit("@length(max: %d)\n", AsInt(max))
	} else if min != nil {
		w.Emit("@length(min: %d)\n", AsInt(min))
	}
}

func (w *IdlWriter) EmitRangeTrait(v interface{}, indent string) {
	l := AsMap(v)
	min := Get(l, "min")
	max := Get(l, "max")
	if min != nil && max != nil {
		w.Emit("@range(min: %v, max: %v)\n", AsDecimal(min), AsDecimal(max))
	} else if max != nil {
		w.Emit("@range(max: %v)\n", AsDecimal(max))
	} else if min != nil {
		w.Emit("@range(min: %v)\n", AsDecimal(min))
	}
}

func (w *IdlWriter) EmitEnumTrait(v interface{}, indent string) {
	en := v.([]interface{})
	if len(en) > 0 {
		s := Pretty(en)
		slen := len(s)
		if slen > 0 && s[slen-1] == '\n' {
			s = s[:slen-1]
		}
		w.Emit("@enum(%s)\n", s)
	}
}

func (w *IdlWriter) EmitTraitTrait(v interface{}) {
	l := AsMap(v)
	if l != nil {
		var lst []string
		selector := GetString(l, "selector")
		if selector != "" {
			lst = append(lst, fmt.Sprintf("selector: %q", selector))
		}
		conflicts := GetStringArray(l, "conflicts")
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
		structurallyExclusive := GetString(l, "structurallyExclusive")
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
	/*
		if dep != nil {
			s := indent + "@deprecated"
			if dep.Message != "" {
				s = s + fmt.Sprintf("(message: %q", dep.Message)
			}
			if dep.Since != "" {
				if s == "@deprecated" {
					s = s + fmt.Sprintf("(since: %q)", dep.Since)
				} else {
					s = s + fmt.Sprintf(", since: %q)", dep.Since)
				}
			}
			w.Emit(s+"\n")
		}
	*/
	panic("fix me")
}

func (w *IdlWriter) EmitHttpTrait(rv interface{}, indent string) {
	var method, uri string
	code := 0
	switch v := rv.(type) {
	case map[string]interface{}:
		method = GetString(v, "method")
		uri = GetString(v, "uri")
		code = GetInt(v, "code")
	case *Struct:
		method = AsString(v.Get("method"))
		uri = AsString(v.Get("uri"))
		code = AsInt(v.Get("code"))
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

func (w *IdlWriter) EmitSimpleShape(shapeName, name string) {
	w.Emit("%s %s\n", shapeName, name)
}

func (w *IdlWriter) EmitBooleanShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.EmitSimpleShape("boolean", name)
}

func (w *IdlWriter) EmitNumericShape(shapeName, name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.EmitSimpleShape(shapeName, name)
}

func (w *IdlWriter) EmitStringShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("%s %s\n", shape.Type, name)
}

func (w *IdlWriter) EmitTimestampShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("timestamp %s\n", name)
}

func (w *IdlWriter) EmitBlobShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("blob %s\n", name)
}

func (w *IdlWriter) EmitCollectionShape(shapeName, name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("%s %s {\n", shapeName, name)
	w.Emit("    member: %s\n", w.stripNamespace(shape.Member.Target))
	w.Emit("}\n")
}

func (w *IdlWriter) EmitMapShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("map %s {\n    key: %s,\n    value: %s\n}\n", name, w.stripNamespace(shape.Key.Target), w.stripNamespace(shape.Value.Target))
}

func (w *IdlWriter) EmitUnionShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("union %s {\n", name)
	count := len(shape.Members)
	for _, fname := range shape.memberKeys {
		mem := shape.Members[fname]
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

func (w *IdlWriter) EmitTraits(traits *Struct, indent string) {
	//note: documentation has an alternate for ("///"+comment), but then must be before other traits.
	if traits == nil {
		return
	}
	for _, k := range traits.Keys() {
		v := traits.Get(k)
		switch k {
		case "smithy.api#documentation":
			w.EmitDocumentation(AsString(v), indent)
		}
	}
	for _, k := range traits.Keys() {
		v := traits.Get(k)
		switch k {
		case "smithy.api#documentation", "smithy.api#examples":
			//do nothing
		case "smithy.api#sensitive", "smithy.api#required", "smithy.api#readonly", "smithy.api#idempotent":
			w.EmitBooleanTrait(AsBool(v), w.stripNamespace(k), indent)
		case "smithy.api#httpLabel", "smithy.api#httpPayload":
			w.EmitBooleanTrait(AsBool(v), w.stripNamespace(k), indent)
		case "smithy.api#httpQuery", "smithy.api#httpHeader":
			w.EmitStringTrait(AsString(v), w.stripNamespace(k), indent)
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
			w.EmitStringTrait(AsString(v), w.stripNamespace(k), indent)
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
	if m, ok := v.(*Struct); ok {
		if m.Length() > 0 {
			var lst []string
			for _, ak := range m.Keys() {
				av := m.Bindings[ak]
				lst = append(lst, fmt.Sprintf("%s: %s", ak, Json(av)))
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
	switch data := raw.(type) {
	case []map[string]interface{}:
		target := w.stripNamespace(opname)
		formatted := Pretty(data)
		if strings.HasSuffix(formatted, "\n") {
			formatted = formatted[:len(formatted)-1]
		}
		w.Emit("apply "+target+" @examples(%s)\n", formatted)
	default:
		panic("FIX ME!")
	}
}

func (w *IdlWriter) EmitStructureShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("structure %s {\n", name)
	for i, k := range shape.memberKeys {
		if i > 0 {
			w.Emit("\n")
		}
		v := shape.Members[k]
		w.EmitTraits(v.Traits, IndentAmount)
		w.Emit("%s%s: %s,\n", IndentAmount, k, w.stripNamespace(v.Target))
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
	w.EmitTraits(shape.Traits, "")
	w.Emit("service %s {\n", name)
	w.Emit("    version: %q,\n", shape.Version)
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
	w.Emit("resource %s {\n", name)
	if len(shape.Identifiers) > 0 {
		w.Emit("    identifiers: {\n")
		for k, v := range shape.Identifiers {
			w.Emit("        %s: %s,\n", k, v.Target) //fixme
		}
		w.Emit("    }\n")
		if shape.Create != nil {
			w.Emit("    create: %v\n", shape.Create)
		}
		if shape.Put != nil {
			w.Emit("    put: %v\n", shape.Put)
		}
		if shape.Read != nil {
			w.Emit("    read: %v\n", shape.Read)
		}
		if shape.Update != nil {
			w.Emit("    update: %v\n", shape.Update)
		}
		if shape.Delete != nil {
			w.Emit("    delete: %v\n", shape.Delete)
		}
		if shape.List != nil {
			w.Emit("    list: %v\n", shape.List)
		}
		if len(shape.Operations) > 0 {
			w.Emit("    %s\n", w.listOfShapeRefs("operations", "%s", shape.Operations, true))
		}
		if len(shape.CollectionOperations) > 0 {
			w.Emit("    %s\n", w.listOfShapeRefs("collectionOperations", "%s", shape.CollectionOperations, true))
		}
	}
	w.Emit("}\n")
}

func (w *IdlWriter) EmitOperationShape(name string, shape *Shape) {
	w.EmitTraits(shape.Traits, "")
	w.Emit("operation %s {\n", name)
	if shape.Input != nil {
		w.Emit("    input: %s,\n", w.stripNamespace(shape.Input.Target))
	}
	if shape.Output != nil {
		w.Emit("    output: %s,\n", w.stripNamespace(shape.Output.Target))
	}
	if len(shape.Errors) > 0 {
		w.Emit("    %s,\n", w.listOfShapeRefs("errors", "%s", shape.Errors, false))
	}
	w.Emit("}\n")
}

func (w *IdlWriter) End() string {
	w.writer.Flush()
	return w.buf.String()
}
