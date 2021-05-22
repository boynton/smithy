package smithy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Model struct {
	ast *AST
}

func (model *Model) String() string {
	return Pretty(model.ast)
}

func AssembleModel(paths []string) (*Model, error) {
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
	err = assembly.Validate()
	if err != nil {
		return nil, err
	}
	return &Model{ast: assembly}, nil
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

func (model *Model) Generate(genName string, outdir string) error {
	gen, err := model.Generator(genName)
	if err != nil {
		return err
	}
	return gen.Generate(model, outdir)
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
