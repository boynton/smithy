package smithy

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boynton/data"
)

type Generator interface {
	Generate(ast *AST, config *data.Object) error
}

type BaseGenerator struct {
	Config         *data.Object
	OutDir         string
	ForceOverwrite bool
	buf            bytes.Buffer
	file           *os.File
	writer         *bufio.Writer
	Err            error
}

func (gen *BaseGenerator) Configure(conf *data.Object) error {
	gen.Config = conf
	gen.OutDir = conf.GetString("outdir")
	gen.ForceOverwrite = conf.GetBool("force")
	return nil
}

func (gen *BaseGenerator) FileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func (gen *BaseGenerator) FileName(ns string, suffix string) string {
	return strings.ReplaceAll(ns, ".", "-") + suffix
}

func (gen *BaseGenerator) WriteFile(path string, content string) error {
	if gen.Err != nil {
		return gen.Err
	}
	if !gen.ForceOverwrite && gen.FileExists(path) {
		return fmt.Errorf("[%s already exists, not overwriting]", path)
	}
	f, err := os.Create(path)
	if err != nil {
		gen.Err = err
		return err
	}
	defer f.Close()
	writer := bufio.NewWriter(f)
	_, gen.Err = writer.WriteString(content)
	writer.Flush()
	return err
}

func (gen *BaseGenerator) Emit(text string, filename string, separator string) error {
	if gen.OutDir == "" {
		if separator != "" {
			fmt.Print(separator)
		}
		fmt.Print(text)
	} else {
		fpath := filepath.Join(gen.OutDir, filename)
		err := gen.WriteFile(fpath, text)
		if err != nil {
			return err
		}
	}
	return nil
}

type AstGenerator struct {
	BaseGenerator
}

func (gen *AstGenerator) Generate(ast *AST, config *data.Object) error {
	err := gen.Configure(config)
	if err != nil {
		return err
	}
	text := data.Pretty(ast)
	return gen.Emit(text, "model.json", "")
}

type IdlGenerator struct {
	BaseGenerator
}

func (gen *IdlGenerator) Generate(ast *AST, config *data.Object) error {
	err := gen.Configure(config)
	if err != nil {
		return err
	}
	//generate one file per namespace. For outdir == "", concatenate with separator indicating intended filename
	//fixme: preserve metadata. Smithy IDL is problematic for that, since metadata is not namespaced, and gets merged
	//on assembly. Should each namespaced IDL get all metadata? none?
	for _, ns := range ast.Namespaces() {
		fname := gen.FileName(ns, ".smithy")
		sep := fmt.Sprintf("\n// ===== File(%q)\n\n", fname)
		s := ast.IDL(ns)
		err := gen.Emit(s, fname, sep)
		if err != nil {
			return err
		}
	}
	return nil
}
