package main

import (
	"encoding/json"
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

	node, err := parser.GetAst(b, &parser.Options{SourceType: "module"}, 0)
	if err != nil {
		println("Error while parsing file")
		log.Fatal(err)
	}

	bJson, _ := json.MarshalIndent(node, "", "  ")
	println(string(bJson))
}
