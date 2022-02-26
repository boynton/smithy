package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boynton/data"
	"github.com/boynton/smithy"
)

func main() {
	conf := data.NewObject()
	pVersion := flag.Bool("v", false, "Show api tool version and exit")
	pList := flag.Bool("l", false, "Show only the list of shape names")
	pForce := flag.Bool("f", false, "Force overwrite if output file exists")
	pGen := flag.String("g", "idl", "The generator for output")
	pOutdir := flag.String("o", "", "The directory to generate output into (defaults to stdout)")
	var params Params
	flag.Var(&params, "a", "Additional named arguments for a generator")
	var tags Tags
	flag.Var(&tags, "t", "Tag of shapes to include")

	flag.Parse()
	if *pVersion {
		fmt.Printf("Smithy tool %s [%s]\n", smithy.ToolVersion, "https://github.com/boynton/smithy")
		os.Exit(0)
	}
	gen := *pGen
	outdir := *pOutdir
	files := flag.Args()
	if len(files) == 0 {
		fmt.Println("usage: smithy [-v] [-o outfile] [-g generator] [-a key=val]* file ...")
		flag.PrintDefaults()
		os.Exit(1)
	}
	ast, err := AssembleModel(files, tags)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	if *pList {
		for _, n := range ast.ShapeNames() {
			fmt.Println(n)
		}
		os.Exit(0)
	}
	conf.Put("outdir", outdir)
	conf.Put("force", *pForce)
	for _, a := range params {
		kv := strings.Split(a, "=")
		if len(kv) > 1 {
			conf.Put(kv[0], kv[1])
		} else {
			conf.Put(a, true)
		}
	}
	generator, err := Generator(gen)
	if err == nil {
		err = generator.Generate(ast, conf)
	}
	if err != nil {
		fmt.Printf("*** %v\n", err)
		os.Exit(4)
	}
}

type Params []string

func (p *Params) String() string {
	return strings.Join([]string(*p), " ")
}
func (p *Params) Set(value string) error {
	*p = append(*p, strings.TrimSpace(value))
	return nil
}

type Tags []string

func (p *Tags) String() string {
	return strings.Join([]string(*p), " ")
}
func (p *Tags) Set(value string) error {
	*p = append(*p, strings.TrimSpace(value))
	return nil
}

func Generator(genName string) (smithy.Generator, error) {
	switch genName {
	case "ast":
		return new(smithy.AstGenerator), nil
	case "idl":
		return new(smithy.IdlGenerator), nil
	case "sadl":
		return new(smithy.SadlGenerator), nil
	default:
		return nil, fmt.Errorf("Unknown generator: %q", genName)
	}
}

func AssembleModel(paths []string, tags []string) (*smithy.AST, error) {
	flatPathList, err := expandPaths(paths)
	if err != nil {
		return nil, err
	}
	assembly := &smithy.AST{
		Smithy: "1.0",
	}
	for _, path := range flatPathList {
		var ast *smithy.AST
		var err error
		ext := filepath.Ext(path)
		switch ext {
		case ".json":
			ast, err = smithy.LoadAST(path)
		case ".smithy":
			ast, err = smithy.Parse(path)
		default:
			return nil, fmt.Errorf("parse for file type %q not implemented", ext)
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
	return assembly, nil
}

var ImportFileExtensions = map[string][]string{
	".smithy": []string{"smithy"},
	".json":    []string{"smithy"},
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
