package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"go_js/compiler"
	"go_js/eventloop"
	"go_js/flags"
	"go_js/heap"
	"go_js/native"
	"go_js/parser"
	"go_js/queue"
	virtualMachine "go_js/vm"
	"log"
	"os"
	"runtime/pprof"
	"strings"
	"sync"
)

var PROFILE = false
var ROOT_FILE_OCATION string

type structuredOut struct {
	Output string         `json:"output"`
	Code   map[int]string `json:"code"`
	Ast    *parser.Node   `json:"ast"`
}

func main() {
	if PROFILE {
		f, err := os.Create("cpu.prof")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create CPU profile: %v\n", err)
			os.Exit(1)
		}

		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Could not start CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <input_file>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.BoolVar(&flags.Debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&flags.StructuredOutput, "structured", false, "Enable structured output")

	flag.Parse()

	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: go run main.go [options] <input>")
		flag.PrintDefaults()
		os.Exit(1)
	}
	input := args[0]

	b, err := os.ReadFile(input)
	split := strings.Split(input, "/")

	rootFileLocation := strings.Join(split[:len(split)-1], "/")

	virtualMachine.InitFileRoot(rootFileLocation)
	compiler.InitRootScriptLocation(rootFileLocation)

	if err != nil {
		println("Can't read file: ", os.Args[1])
		os.Exit(1)
	}

	ast, err := parser.GetAst(b, &parser.Options{SourceType: "module"}, 0)

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	if flags.Debug {
		println("### Abtract Syntax Tree ###")
		parser.PrintNode(ast)
		println()
	}
	native.Init()

	main, err := compiler.Compile(ast)
	heap.InitGC(main.ValueChunk().Constants)

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	var wg sync.WaitGroup

	queue.Init(&wg)
	eventloop.Init(&wg)

	var output *strings.Builder

	if flags.StructuredOutput {
		output = &strings.Builder{}
	}

	vm := virtualMachine.NewVM(true, output)

	go eventloop.Start()
	go vm.Run(&wg)

	mainJob := &native.Main{Fn: main}
	eventloop.Dispatch(mainJob)

	wg.Wait()

	if flags.StructuredOutput {
		sb := &strings.Builder{}
		r := virtualMachine.StructureOutput(*main.ValueChunk(), sb)

		out := structuredOut{
			Output: output.String(),
			Code:   r,
			Ast:    ast,
		}

		res, err := json.Marshal(out)

		if err != nil {
			log.Fatalf("Failed to marshal output %s", err.Error())
		}

		str := base64.StdEncoding.EncodeToString(res)

		fmt.Printf("%s", str)
	}
}
