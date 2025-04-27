package main

import (
	"go_js/parser"
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

	node, err := parser.GetAst(b, nil, 0)
	if err != nil {
		println("Error while parsing file")
		log.Fatal(err)
	}

	parser.PrintNode(node)
}
