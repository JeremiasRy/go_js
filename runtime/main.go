package main

import (
	"fmt"
	"go_js/compiler"
	eventloop "go_js/eventLoop"
	"go_js/native"
	"go_js/parser"
	"go_js/queue"
	"go_js/vm"
	"log"
	"os"
	"runtime/pprof"
	"sync"
)

var PROFILE = false

func main() {
	var debug bool = false
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

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <input>")
		os.Exit(1)
	}

	if len(os.Args) == 3 {
		debug = os.Args[2] == "--debug"
	}

	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("Can't read file: ", os.Args[1])
		os.Exit(1)
	}

	ast, err := parser.GetAst(b, nil, 0)

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	if debug {
		fmt.Println("### Abtract Syntax Tree ###")
		parser.PrintNode(ast)
		fmt.Println()
	}

	main, err := compiler.Compile(ast)

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}
	var wg sync.WaitGroup

	queue.Init(&wg)
	eventloop.Init(&wg)

	vm := vm.NewVM(debug)

	go eventloop.Start()
	go vm.Run(&wg)

	mainJob := &native.Main{Fn: main}
	wg.Add(1)
	eventloop.DispatchJob(mainJob)

	wg.Wait()

	if err != nil {
		log.Fatalf("runtime error: %s", err.Error())
	}

}
