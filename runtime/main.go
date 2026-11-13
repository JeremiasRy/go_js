package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"go_js/compiler"
	"go_js/diagnostic"
	"go_js/eventloop"
	"go_js/flags"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
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
	flag.BoolVar(&flags.JSONErrors, "json-errors", false, "Report error classes and phases as JSON on stderr")
	sourceType := flag.String("source-type", "module", "JavaScript source goal: script or module")
	jitMode := flag.String("jit", flags.JITMode, "JIT mode: off, auto or force")
	flag.IntVar(&flags.JITCallThreshold, "jit-call-threshold", flags.JITCallThreshold, "Call count before a function is JIT compiled")
	flag.IntVar(&flags.JITLoopThreshold, "jit-loop-threshold", flags.JITLoopThreshold, "Backedge count before a hot loop is compiled with OSR")
	flag.IntVar(&flags.JITMaxCodeBytes, "jit-max-code-bytes", flags.JITMaxCodeBytes, "Maximum executable code cache size in bytes")
	flag.BoolVar(&flags.JITStats, "jit-stats", false, "Print JIT statistics on exit")
	flag.BoolVar(&flags.JITTrace, "jit-trace", false, "Trace JIT compilation and execution")

	flag.Parse()
	if *sourceType != "script" && *sourceType != "module" {
		diagnostic.Fatal("configuration", "Error", fmt.Errorf("invalid source type %q", *sourceType))
	}
	if err := flags.SetJITMode(*jitMode); err != nil {
		diagnostic.Fatal("configuration", "Error", err)
	}

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
	compiler.SetEntryFile(input)
	virtualMachine.SetScriptMode(*sourceType == "script")
	compiler.SetModule(*sourceType == "module")

	if err != nil {
		println("Can't read file: ", os.Args[1])
		os.Exit(1)
	}

	ast, err := parser.GetAst(b, &parser.Options{SourceType: *sourceType}, 0)

	if err != nil {
		diagnostic.Fatal("parse", "SyntaxError", err)
	}

	if flags.Debug {
		println("### Abtract Syntax Tree ###")
		parser.PrintNode(ast)
		println()
	}
	native.Init()

	main, err := compiler.Compile(ast)

	if err != nil {
		if linkErr, ok := err.(*compiler.ModuleLinkError); ok {
			class := linkErr.Class
			if class == "" {
				class = "SyntaxError"
			}
			diagnostic.Fatal("link", class, err)
		}
		diagnostic.Fatal("compile", "Error", err)
	}
	heap.InitGC(main.ValueChunk().Constants)

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

	if flags.JITStats {
		virtualMachine.PrintJITStats(os.Stderr)
	}

	if flags.StructuredOutput {
		info := virtualMachine.NewDebugInfo()
		info.AddCallable(main, "", false)
		entry := compiler.EntryModule()
		for _, record := range compiler.ModuleRecords() {
			if record.Main == nil {
				continue
			}
			nameOverride := ""
			hideSameName := false
			if record == entry {
				nameOverride = object.MAIN_FN_NAME
				hideSameName = true
			}
			info.AddCallable(record.Main, nameOverride, hideSameName)
		}
		info.Finalize()

		out := virtualMachine.StructuredOut{
			FormatVersion: virtualMachine.StructuredOutputFormatVersion,
			Output:        output.String(),
			DebugChunks:   info.Chunks,
			Ast:           ast,
			Execution:     virtualMachine.NewExecutionPlaceholder(),
		}

		res, err := json.Marshal(out)

		if err != nil {
			log.Fatalf("Failed to marshal output %s", err.Error())
		}

		str := base64.StdEncoding.EncodeToString(res)

		fmt.Printf("%s", str)
	}
}
