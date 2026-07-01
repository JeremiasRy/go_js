package structuredout

import (
	"encoding/json"
	"fmt"
	"go_js/parser"
	"log"
	"strings"
)

type StructuredOutputEntry struct {
	Op    uint8 `json:"op"`
	AstId int   `json:"ast_id"`
}

type StructuredOutput struct {
	Ast     *parser.Node                       `json:"ast"`
	OpCodes map[string][]StructuredOutputEntry `json:"op_codes"`
	Output  string                             `json:"output"`
}

const UNINITIALIZED_AST_ID = -1

var currentFn = ""
var currentAstId = UNINITIALIZED_AST_ID
var astOp = map[string][]StructuredOutputEntry{}
var astJSON *parser.Node = nil
var outputBuffer *strings.Builder = &strings.Builder{}

func AppendOpCode(op ...uint8) {
	if _, ok := astOp[currentFn]; !ok || currentAstId == UNINITIALIZED_AST_ID {
		log.Fatalf("most prob un-initialized structured output situation %s", currentFn)
	}
	entries := []StructuredOutputEntry{}

	for _, code := range op {
		entries = append(entries, StructuredOutputEntry{Op: code, AstId: currentAstId})
	}
	astOp[currentFn] = append(astOp[currentFn], entries...)
}

func PatchOpCodes(from int, code []uint8) {
	for idx, op := range code {
		astOp[currentFn][from+idx] = StructuredOutputEntry{Op: op, AstId: currentAstId}
	}
}

func WriteToOutputBuffer(i string) {
	fmt.Fprintf(outputBuffer, "%s\n", i)
}

func SetCurrentFn(fn string) {
	currentFn = fn
	astOp[currentFn] = []StructuredOutputEntry{}
}

func SetCurrentAstId(id int) {
	currentAstId = id
}

func SetAstJSON(ast *parser.Node) {
	astJSON = ast
}

func ReturnStructuredOutput() ([]byte, error) {
	return json.Marshal(StructuredOutput{
		Ast:     astJSON,
		OpCodes: astOp,
		Output:  outputBuffer.String(),
	})
}
