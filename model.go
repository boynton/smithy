package smithy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/boynton/smithy/data"
)

type Model struct {
	ast *AST
}

func (model *Model) String() string {
	return data.Pretty(model.ast)
}

func (model *Model) GetAst() *AST {
	return model.ast
}

func AssembleModel(paths []string, tags []string) (*Model, error) {
	flatPathList, err := expandPaths(paths)
	if err != nil {
		return nil, err
	}
	assembly := &AST{
		Smithy: "1.0",
	}
	for _, path := range flatPathList {
		var ast *AST
		var err error
		ext := filepath.Ext(path)
		switch ext {
		case ".json":
			ast, err = loadAST(path)
		case ".smithy":
			ast, err = parse(path) //FIXME: the parser's "use" map is lost here. Would be useful for unparse!
		default:
			return nil, fmt.Errorf("Unrecognized file type: %q", ext)
		}
		if err != nil {
			return nil, err
		}
		err = assembly.Merge(ast)
		if err != nil {
			return nil, err
		}
	}
	if len(tags) > 0 {
		assembly.Filter(tags)
	}
	err = assembly.Validate()
	if err != nil {
		return nil, err
	}
	return &Model{ast: assembly}, nil
}

func containsString(ary []string, val string) bool {
	for _, s := range ary {
		if s == val {
			return true
		}
	}
	return false
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
	filtered := newShapes()
	for name, _ := range included {
		filtered.Put(name, ast.GetShape(name))
	}
	ast.Shapes = filtered
}

func (ast *AST) noteDependenciesFromRef(included map[string]bool, ref *ShapeRef) {
	if ref != nil {
		ast.noteDependencies(included, ref.Target)
	}
}

func (ast *AST) noteDependencies(included map[string]bool, name string) {
	//note traits
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
	default:
		fmt.Println("HANDLE THIS:", shape.Type)
		//		panic("whoa")
	}
}

func (ast *AST) Validate() error {
	//todo
	return nil
}

func (ast *AST) Merge(src *AST) error {
	if ast.Smithy != src.Smithy {
		return fmt.Errorf("Smithy version mismatch. Expected %s, got %s\n", ast.Smithy, src.Smithy)
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

func loadAST(path string) (*AST, error) {
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

var ImportFormats = []string{
	"smithy",
	//	"openapi",
	//	"sadl",
	//	"graphql",
}

var ImportFileExtensions = map[string][]string{
	".smithy": []string{"smithy"},
	//".json":    []string{"smithy", "openapi"},
	".json": []string{"smithy"},
}

func expandPaths(paths []string) ([]string, error) {
	var result []string
	for _, path := range paths {
		ext := filepath.Ext(path)
		if _, ok := ImportFileExtensions[ext]; ok {
			result = append(result, path)
		} else {
			fi, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			if fi.IsDir() {
				err = filepath.Walk(path, func(wpath string, info os.FileInfo, errIncoming error) error {
					if errIncoming != nil {
						return errIncoming
					}
					ext := filepath.Ext(wpath)
					if _, ok := ImportFileExtensions[ext]; ok {
						result = append(result, wpath)
					}
					return nil
				})
			}
		}
	}
	return result, nil
}

func shapeIdNamespace(id string) string {
	//name.space#entity$member
	lst := strings.Split(id, "#")
	return lst[0]
}

func (model *Model) ShapeNames() []string {
	var lst []string
	for _, k := range model.ast.Shapes.Keys() {
		lst = append(lst, k)
	}
	return lst
}

func (model *Model) Namespaces() []string {
	m := make(map[string]int, 0)
	if model.ast.Shapes != nil {
		for _, id := range model.ast.Shapes.Keys() {
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

func (model *Model) Generate(genName string, conf *data.Object) error {
	gen, err := model.Generator(genName)
	if err != nil {
		return err
	}
	return gen.Generate(model, conf)
}

func (model *Model) Generator(genName string) (Generator, error) {
	switch genName {
	case "ast":
		return new(AstGenerator), nil
	case "idl":
		return new(IdlGenerator), nil
	default:
		return nil, fmt.Errorf("Unknown generator: %q", genName)
	}
}
