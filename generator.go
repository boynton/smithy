package smithy

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Generator interface {
	Generate(model *Model, config *Data) error
}

type BaseGenerator struct {
	Model          *Model
	Config         *Data
	OutDir         string
	ForceOverwrite bool
	buf            bytes.Buffer
	file           *os.File
	writer         *bufio.Writer
	Err            error
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

type AstGenerator struct {
	BaseGenerator
}

func (gen *AstGenerator) Generate(model *Model, config *Data) error {
	outdir := config.GetString("outdir")
	gen.Model = model
	fname := "model.json"
	s := Pretty(gen.Model.ast)
	if outdir == "" {
		fmt.Print(s)
	} else {
		fpath := filepath.Join(outdir, fname)
		err := gen.WriteFile(fpath, s)
		if err != nil {
			return err
		}
	}
	return nil
}

type IdlGenerator struct {
	BaseGenerator
}

func (gen *IdlGenerator) Generate(model *Model, config *Data) error {
	outdir := config.GetString("outdir")
	//generate one file per namespace. For outdir == "", concatenate with separator indicating intended filename
	gen.Model = model
	//fixme: preserve metadata. Smithy IDL is problematic for that, since metadata is not namespaced, and gets merged
	//on assembly. Should each namespaced IDL get all metadata? none?
	for _, ns := range gen.Model.Namespaces() {
		fname := gen.FileName(ns, ".smithy")
		s := gen.Model.ast.IDL(ns)
		if outdir == "" {
			fmt.Printf("\n// ===== File(%q)\n\n%s", fname, s) //note: the combined output is not a valid Smithy IDL file
		} else {
			fpath := filepath.Join(outdir, fname)
			err := gen.WriteFile(fpath, s)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
