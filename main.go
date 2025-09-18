package main

import (
	"fmt"
	"go_js/compiler"
	eventloop "go_js/eventLoop"
	"go_js/object"
	"go_js/parser"
	"go_js/vm"
	"log"
	"os"
	"runtime/pprof"
	"sync"
	"time"
)

const PROFILE = true
const DEBUG = true

var WAIT_GROUP = sync.WaitGroup{}

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

	if len(os.Args) != 2 {
		println("Usage: go run main.go <input>")
		os.Exit(1)
	}

	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		println("Can't read file: ", os.Args[1])
		os.Exit(1)
	}

	startAstParse := time.Now()
	ast, err := parser.GetAst(b, nil, 0)
	fmt.Printf("AST parsed in %s\n", time.Since(startAstParse))

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	if DEBUG {
		println("### Abtract Syntax Tree ###")
		parser.PrintNode(ast)
		println()
	}

	startCompile := time.Now()
	main, err := compiler.Compile(ast)
	fmt.Printf("AST Compiled in %s\n", time.Since(startCompile))

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}
	var wg sync.WaitGroup

	jobChannel := make(chan object.Job)

	start := time.Now()
	vm := vm.NewVM(DEBUG, jobChannel)

	eventloop.InitLoop(jobChannel, vm)
	go eventloop.Start(&wg)
	wg.Add(1) // for initializing
	mainJob := &object.Main{Fn: main}
	jobChannel <- mainJob
	wg.Done()

	wg.Wait()
	fmt.Printf("Thanks! %s\n", time.Since(start))

	if err != nil {
		log.Fatalf("runtime error: %s", err.Error())
	}

}
