package main

import (
	"go_js/vm"
	"log"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		println("Usage: go run main.go <input>")
		os.Exit(1)
	}

	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		println("Can't read file: ", os.Args[1])
		os.Exit(1)
	}

	err = vm.Interpret(b)

	if err != nil {
		log.Fatalf("Runtime error -%e-", err)
	}
}
