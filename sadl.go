package smithy

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/boynton/smithy/data"
)

type SadlGenerator struct {
	BaseGenerator
}

func (gen *SadlGenerator) Generate(model *Model, config *data.Object) error {
	err := gen.Configure(config)
	if err != nil {
		return err
	}
	ns := config.GetString("namespace")
	if ns == "" {
		lstNs := model.Namespaces()
		if len(lstNs) == 1 {
			ns = lstNs[0]
		} else {
			return fmt.Errorf("Multiple namespaces in smithy model, SADL requires that you choose one")
		}
	}
	fname := gen.FileName(ns, ".sadl")
	s := gen.ToSadl(ns, model)
	return gen.Emit(s, fname, "")
}

type SadlWriter struct {
	buf       bytes.Buffer
	writer    *bufio.Writer
	namespace string
	name      string
	model     *Model
	config    *data.Object
}

func (gen *SadlGenerator) ToSadl(ns string, model *Model) string {
	ast := model.ast
	w := &SadlWriter{
		namespace: ns,
		model:     model,
		config:    gen.Config,
	}
	emitted := make(map[string]bool, 0)

	w.Begin()

	//output service attributes, then actions, then types

	//SADL currently handles a single service. So find the first one and use that...
	//should I shake the tree to only include types/operations/resources that are part of that service?
	//I think by default, I want to see everything.
	//bool includeAll := true
	serviceName, _ := gen.serviceName(model, ns)
	if serviceName != "" {
		w.Emit("name %s\n", serviceName)
		emitted[serviceName] = true
	}
	if ns != "" {
		w.Emit("namespace %s\n", ns)
	}

	/*
		imports := ast.ExternalRefs(ns)
		if len(imports) > 0 {
			w.Emit("\n")
			for _, im := range imports {
				if im == "smithy.api#Document" {
					w.Emit("type Document Struct\n")
				}
				//w.Emit("use %s\n", im)
			}
		}
	*/
	w.Emit("\n")

	for _, nsk := range ast.Shapes.Keys() {
		lst := strings.Split(nsk, "#")
		//		if lst[0] == ns {
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
		//		}
	}
	for _, nsk := range ast.Shapes.Keys() {
		lst := strings.Split(nsk, "#")
		k := lst[1]
		if !emitted[k] {
			w.EmitShape(k, ast.GetShape(nsk))
		}
	}
	/*
		for _, nsk := range ast.Shapes.Keys() {
			shape := ast.GetShape(nsk)
			if shape.Type == "operation" {
				if d := shape.Traits.Get("smithy.api#examples"); d != nil {
					switch v := d.(type) {
					case []map[string]interface{}:
						//w.EmitExamplesTrait(nsk, v)
						fmt.Println("FIX ME: example", v)
					}
				}
			}
		}
	*/
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
	var enumItems []interface{}

	var opts []string
	if shape.Traits != nil {
		for _, k := range shape.Traits.Keys() {
			switch k {
			case "smithy.api#tags":
				if w.config.GetBool("annotate") {
					opts = append(opts, fmt.Sprintf("x_tags=%q", strings.Join(shape.Traits.GetStringArray(k), ",")))
				}
			case "smithy.api#enum":
				enumItems = shape.Traits.GetArray("smithy.api#enum")
			case "smithy.api#idempotent", "smithy.api#readonly":
				if w.config.GetBool("annotate") {
					opts = append(opts, "x_"+w.stripNamespace(k))
				}
			default:
				if strings.HasPrefix(k, "smithy.api#") {
					//ignore, i.e. things like http, httpError, etc, they are handled elsewhere
				} else {
					if w.config.GetBool("annotate") {
						opts = append(opts, "x_"+w.stripNamespace(k))
					}
				}
			}
		}
	}
	if len(opts) != 0 {
		fmt.Println("opts for", name, "("+strings.Join(opts, ", ")+")")
	}
	if enumItems != nil {
		w.EmitEnum(name, shape, enumItems)
		return
	}
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
	case "bigInteger":
	case "bigdecimal":
		w.EmitNumericShape("Decimal", name, shape)
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

func (w *SadlWriter) EmitEnum(name string, shape *Shape, lst []interface{}) {
	w.EmitShapeComment(shape)
	w.Emit("type %s Enum {\n", name)
	for _, r := range lst {
		if m, ok := r.(map[string]interface{}); ok {
			if v, ok := m["name"]; ok {
				if s, ok := v.(string); ok {
					//just use the name, ignore the value.
					w.Emit("    %s\n", s)
				}
			}
		}
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
	opts := "" //fixme
	w.Emit("type %s %s (%s)\n", name, shapeName, opts)
}

func (w *SadlWriter) EmitStringShape(name string, shape *Shape) {
	w.EmitShapeComment(shape)
	var opts []string
	pat := shape.Traits.GetString("smithy.api#pattern")
	if pat != "" {
		opts = append(opts, fmt.Sprintf("pattern=%q", pat))
	}
	sopts := ""
	if len(opts) > 0 {
		sopts = " (" + strings.Join(opts, ", ") + ")"
	}
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
	//	w.EmitTraits(shape.Traits, "")
	w.Emit("type %s Array<%s> // %s\n", name, w.stripNamespace(shape.Member.Target), shapeName)
}

func (w *SadlWriter) EmitMapShape(name string, shape *Shape) {
	w.EmitShapeComment(shape)
	//	w.EmitTraits(shape.Traits, "")
	w.Emit("type %s Map<%s,%s>\n", name, w.stripNamespace(shape.Key.Target), w.stripNamespace(shape.Value.Target))
}

func (w *SadlWriter) EmitStructureShape(name string, shape *Shape) {
	w.EmitShapeComment(shape)
	opt := ""
	w.Emit("type " + name + " Struct" + opt + " {\n")
	for _, k := range shape.Members.Keys() {
		v := shape.Members.Get(k)
		tref := w.stripNamespace(w.shapeRefToTypeRef(v.Target))
		w.Emit("%s%s %s", IndentAmount, k, tref)
		w.EmitTraits(v.Traits, IndentAmount)
		w.Emit("\n")
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
		w.Emit("%s%s %s", IndentAmount, k, tref)
		w.EmitTraits(v.Traits, IndentAmount)
		w.Emit("\n")
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

	opts = append(opts, fmt.Sprintf("action=%s", name))
	sopts := "(" + strings.Join(opts, ", ") + ")"
	queryParams := ""
	var inShape *Shape
	inputIsPayload := method == "PUT" || method == "POST"
	if inType != "" {
		inShape = w.model.ast.GetShape(inType)
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
					p := s + "={" + s + "}"
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
		outShape = w.model.ast.GetShape(outType)
		if outShape.Members.Length() > 0 {
			w.Emit("\texpect %d {\n", expected)
			for _, k := range outShape.Members.Keys() {
				v := outShape.Members.Get(k)
				if v.Traits.Has("smithy.api#httpPayload") {
					//
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
				w.Emit("\t\t%s %s%s\n", k, w.stripNamespace(v.Target), sopts)
			}
			w.Emit("\t}\n")
		} else {
			w.Emit("\texpect %d\n", expected) //no content
		}
	}
	//except: we have to iterate through the "errors" of the operation, and check each one for httpError
	//Note that there is in that case not much opportunity to do headers.
	if len(shape.Errors) > 0 {
		for _, errType := range shape.Errors {
			errShape := w.model.ast.GetShape(errType.Target)
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

func (w *SadlWriter) End() string {
	w.writer.Flush()
	return w.buf.String()
}

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

func (w *SadlWriter) EmitTraits(traits *data.Object, indentAmount string) {
	var opts []string
	if traits != nil {
		if traits.Has("smithy.api#required") {
			opts = append(opts, "required")
		}
		if opts != nil {
			w.Emit(" (%s)", strings.Join(opts, ", "))
		}
	}
}
