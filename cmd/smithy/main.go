package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/boynton/smithy"
)

func main() {
	conf := smithy.NewData()
	pVersion := flag.Bool("v", false, "Show api tool version and exit")
	pGen := flag.String("g", "idl", "The generator for output")
	pOutdir := flag.String("o", "", "The directory to generate output into (defaults to stdout)")
	flag.Parse()
	if *pVersion {
		fmt.Printf("Smithy tool %s [%s]\n", smithy.ToolVersion, "https://github.com/boynton/smithy")
		os.Exit(0)
	}
	gen := *pGen
	outdir := *pOutdir
	files := flag.Args()
	if len(files) == 0 {
		fmt.Println("usage: smithy [-v] [-o outfile] [-g generator] file ...")
		flag.PrintDefaults()
		os.Exit(1)
	}
	model, err := smithy.AssembleModel(files)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	conf.Put("outdir", outdir)
	generator, err := Generator(gen)
	if err == nil {
		err = generator.Generate(model, conf)
	}
	if err != nil {
		fmt.Printf("*** %v\n", err)
		os.Exit(4)
	}
}

func Generator(genName string) (smithy.Generator, error) {
	switch genName {
	case "ast":
		return new(smithy.AstGenerator), nil
	case "idl":
		return new(smithy.IdlGenerator), nil
	default:
		return nil, fmt.Errorf("Unknown generator: %q", genName)
	}
}
