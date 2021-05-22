package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/boynton/smithy"
)

func main() {
	pVersion := flag.Bool("v", false, "Show api tool version and exit")
	pGen := flag.String("g", "idl", "The generator for output")
	pOutdir := flag.String("o", "", "The directory to generate output into (defaults to stdout)")
	flag.Parse()
	if *pVersion {
		fmt.Printf("smithy tool %s\n", smithy.ToolVersion)
		os.Exit(0)
	}
	if false {
		s := smithy.NewStruct()
		s.Put("foo", 23)
		s.Put("bar", "blah")
		s.Put("more", []string{"one", "two", "three"})
		fmt.Println("s:", s)
		fmt.Println(smithy.Pretty(s))
		s.Put("hey", true)
		fmt.Println(smithy.Pretty(s))
		fmt.Println(smithy.Pretty(s.Bindings))
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
	err = model.Generate(gen, outdir)
	if err != nil {
		fmt.Printf("*** %v\n", err)
		os.Exit(4)
	}
}
