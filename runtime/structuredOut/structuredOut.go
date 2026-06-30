package structuredout

import (
	"encoding/json"
	"go_js/parser"
	"log"
)

type StructuredOutputEntry struct {
	Op    uint8 `json:"op"`
	AstId int   `json:"ast_id"`
}

type StructuredOutput struct {
	Ast     *parser.Node                       `json:"ast"`
	OpCodes map[string][]StructuredOutputEntry `json:"op_codes"`
}

const UNINITIALIZED_AST_ID = -1

var currentFn = ""
var currentAstId = UNINITIALIZED_AST_ID
var astOp = map[string][]StructuredOutputEntry{}
var astJSON *parser.Node = nil

func AppendOpCode(op ...uint8) {
	if _, ok := astOp[currentFn]; !ok {
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
	})
}
