package parser

import (
	"encoding/json"
	"log"
	"os"
	"reflect"
	"testing"
)

func getTestInput(fileNum string) []byte {
	b, err := os.ReadFile("./test_scripts/test_" + fileNum + ".js")

	if err != nil {
		log.Fatalf("failed to open test file: %s", err.Error())
	}
	return b
}

func areNodesEqual(a *Node, b *Node) bool {
	aJson, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bJson, err := json.Marshal(b)
	if err != nil {
		return false
	}

	return reflect.DeepEqual(aJson, bJson)
}

func TestRecursiveFunction(t *testing.T) {
	input := getTestInput("1")
	expected := &Node{
		Start:      0,
		End:        99,
		Type:       NODE_PROGRAM,
		Range:      [2]int{0, 0},
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Start:      0,
				End:        99,
				Type:       NODE_FUNCTION_DECLARATION,
				Range:      [2]int{0, 0},
				SourceType: TYPE_SCRIPT,
				Identifier: &Node{
					Start:      9,
					End:        13,
					Type:       NODE_IDENTIFIER,
					Range:      [2]int{0, 0},
					Name:       "fibo",
					SourceType: TYPE_SCRIPT,
				},
				Params: []*Node{
					{
						Start:      14,
						End:        15,
						Type:       NODE_IDENTIFIER,
						Range:      [2]int{0, 0},
						Name:       "n",
						SourceType: TYPE_SCRIPT,
					},
				},
				BodyNode: &Node{
					Start:      17,
					End:        99,
					Type:       NODE_BLOCK_STATEMENT,
					Range:      [2]int{0, 0},
					SourceType: TYPE_SCRIPT,
					Body: []*Node{
						{
							Start:      23,
							End:        60,
							Type:       NODE_IF_STATEMENT,
							Range:      [2]int{0, 0},
							SourceType: TYPE_SCRIPT,
							Test: &Node{
								Start:          27,
								End:            34,
								Type:           NODE_BINARY_EXPRESSION,
								Range:          [2]int{0, 0},
								SourceType:     TYPE_SCRIPT,
								BinaryOperator: STRICT_EQUALS,
								Left: &Node{
									Start:      27,
									End:        28,
									Type:       NODE_IDENTIFIER,
									Range:      [2]int{0, 0},
									Name:       "n",
									SourceType: TYPE_SCRIPT,
								},
								Right: &Node{
									Start:      33,
									End:        34,
									Type:       NODE_LITERAL,
									Range:      [2]int{0, 0},
									Raw:        "1",
									Value:      1.0,
									SourceType: TYPE_SCRIPT,
								},
							},
							Consequent: &Node{
								Start:      36,
								End:        60,
								Type:       NODE_BLOCK_STATEMENT,
								Range:      [2]int{0, 0},
								SourceType: TYPE_SCRIPT,
								Body: []*Node{
									{
										Start:      46,
										End:        54,
										Type:       NODE_RETURN_STATEMENT,
										Range:      [2]int{0, 0},
										SourceType: TYPE_SCRIPT,
										Argument: &Node{
											Start:      53,
											End:        54,
											Type:       NODE_IDENTIFIER,
											Range:      [2]int{0, 0},
											Name:       "n",
											SourceType: TYPE_SCRIPT,
										},
									},
								},
							},
						},
						{
							Start:      65,
							End:        97,
							Type:       NODE_RETURN_STATEMENT,
							Range:      [2]int{0, 0},
							SourceType: TYPE_SCRIPT,
							Argument: &Node{
								Start:          72,
								End:            97,
								Type:           NODE_BINARY_EXPRESSION,
								Range:          [2]int{0, 0},
								SourceType:     TYPE_SCRIPT,
								BinaryOperator: PLUS,
								Left: &Node{
									Start:      72,
									End:        83,
									Type:       NODE_CALL_EXPRESSION,
									Range:      [2]int{0, 0},
									SourceType: TYPE_SCRIPT,
									Callee: &Node{
										Start:      72,
										End:        76,
										Type:       NODE_IDENTIFIER,
										Range:      [2]int{0, 0},
										Name:       "fibo",
										SourceType: TYPE_SCRIPT,
									},
									Arguments: []*Node{
										{
											Start:          77,
											End:            82,
											Type:           NODE_BINARY_EXPRESSION,
											Range:          [2]int{0, 0},
											SourceType:     TYPE_SCRIPT,
											BinaryOperator: MINUS,
											Left: &Node{
												Start:      77,
												End:        78,
												Type:       NODE_IDENTIFIER,
												Range:      [2]int{0, 0},
												Name:       "n",
												SourceType: TYPE_SCRIPT,
											},
											Right: &Node{
												Start:      81,
												End:        82,
												Type:       NODE_LITERAL,
												Range:      [2]int{0, 0},
												Raw:        "1",
												Value:      1.0,
												SourceType: TYPE_SCRIPT,
											},
										},
									},
								},
								Right: &Node{
									Start:      86,
									End:        97,
									Type:       NODE_CALL_EXPRESSION,
									Range:      [2]int{0, 0},
									SourceType: TYPE_SCRIPT,
									Callee: &Node{
										Start:      86,
										End:        90,
										Type:       NODE_IDENTIFIER,
										Range:      [2]int{0, 0},
										Name:       "fibo",
										SourceType: TYPE_SCRIPT,
									},
									Arguments: []*Node{
										{
											Start:          91,
											End:            96,
											Type:           NODE_BINARY_EXPRESSION,
											Range:          [2]int{0, 0},
											SourceType:     TYPE_SCRIPT,
											BinaryOperator: MINUS,
											Left: &Node{
												Start:      91,
												End:        92,
												Type:       NODE_IDENTIFIER,
												Range:      [2]int{0, 0},
												Name:       "n",
												SourceType: TYPE_SCRIPT,
											},
											Right: &Node{
												Start:      95,
												End:        96,
												Type:       NODE_LITERAL,
												Range:      [2]int{0, 0},
												Raw:        "2",
												Value:      2.0,
												SourceType: TYPE_SCRIPT,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	actual, err := GetAst(input, nil, 0)
	if err != nil {
		t.Errorf("Failed to generate ast: %s", err)
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal")
	}
}

func TestClosure(t *testing.T) {
	input := getTestInput("2")
	expected := &Node{
		Start:      0,
		End:        140,
		Type:       NODE_PROGRAM,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Start:      0,
				End:        140,
				Type:       NODE_FUNCTION_DECLARATION,
				SourceType: TYPE_SCRIPT,
				Identifier: &Node{
					Start:      9,
					End:        16,
					Type:       NODE_IDENTIFIER,
					Name:       "closure",
					SourceType: TYPE_SCRIPT,
				},
				IsExpression: false,
				IsGenerator:  false,
				IsAsync:      false,
				Params:       []*Node{},
				BodyNode: &Node{
					Start:      19,
					End:        140,
					Type:       NODE_BLOCK_STATEMENT,
					SourceType: TYPE_SCRIPT,
					Body: []*Node{
						{
							Start:      25,
							End:        39,
							Type:       NODE_VARIABLE_DECLARATION,
							SourceType: TYPE_SCRIPT,
							Kind:       KIND_DECLARATION_LET,
							Declarations: []*Node{
								{
									Start:      29,
									End:        38,
									Type:       NODE_VARIABLE_DECLARATOR,
									SourceType: TYPE_SCRIPT,
									Identifier: &Node{
										Start:      29,
										End:        34,
										Type:       NODE_IDENTIFIER,
										Name:       "count",
										SourceType: TYPE_SCRIPT,
									},
									Initializer: &Node{
										Start:      37,
										End:        38,
										Type:       NODE_LITERAL,
										Value:      0.0,
										Raw:        "0",
										SourceType: TYPE_SCRIPT,
									},
								},
							},
						},
						{
							Start:      45,
							End:        138,
							Type:       NODE_RETURN_STATEMENT,
							SourceType: TYPE_SCRIPT,
							Argument: &Node{
								Start:      52,
								End:        138,
								Type:       NODE_OBJECT_EXPRESSION,
								SourceType: TYPE_SCRIPT,
								Properties: []*Node{
									{
										Start:      62,
										End:        132,
										Type:       NODE_PROPERTY,
										SourceType: TYPE_SCRIPT,
										IsMethod:   false,
										Shorthand:  false,
										Computed:   false,
										Kind:       KIND_PROPERTY_INIT,
										Key: &Node{
											Start:      62,
											End:        67,
											Type:       NODE_IDENTIFIER,
											Name:       "count",
											SourceType: TYPE_SCRIPT,
										},
										Value: &Node{
											Start:        69,
											End:          132,
											Type:         NODE_ARROW_FUNCTION_EXPRESSION,
											SourceType:   TYPE_SCRIPT,
											IsExpression: false,
											IsGenerator:  false,
											IsAsync:      false,
											Params:       []*Node{},
											BodyNode: &Node{
												Start:      75,
												End:        132,
												Type:       NODE_BLOCK_STATEMENT,
												SourceType: TYPE_SCRIPT,
												Body: []*Node{
													{
														Start:      89,
														End:        97,
														Type:       NODE_EXPRESSION_STATEMENT,
														SourceType: TYPE_SCRIPT,
														Expression: &Node{
															Start:          89,
															End:            96,
															Type:           NODE_UPDATE_EXPRESSION,
															SourceType:     TYPE_SCRIPT,
															UpdateOperator: "++",
															Prefix:         false,
															Argument: &Node{
																Start:      89,
																End:        94,
																Type:       NODE_IDENTIFIER,
																Name:       "count",
																SourceType: TYPE_SCRIPT,
															},
														},
													},
													{
														Start:      110,
														End:        122,
														Type:       NODE_RETURN_STATEMENT,
														SourceType: TYPE_SCRIPT,
														Argument: &Node{
															Start:      117,
															End:        122,
															Type:       NODE_IDENTIFIER,
															Name:       "count",
															SourceType: TYPE_SCRIPT,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	actual, err := GetAst(input, nil, 0)
	if err != nil {
		t.Errorf("Failed to generate ast: %s", err)
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal")
	}
}

func TestClassPropertyScientificNotation(t *testing.T) {
	input := getTestInput("3")
	expected := &Node{
		Type:  NODE_PROGRAM,
		Start: 0,
		End:   15,
		Body: []*Node{
			{
				Type:  NODE_CLASS_DECLARATION,
				Start: 0,
				End:   15,
				Identifier: &Node{
					Type:  NODE_IDENTIFIER,
					Start: 6,
					End:   7,
					Name:  "C",
				},
				BodyNode: &Node{
					Type:     NODE_CLASS_BODY,
					Start:    8,
					End:      15,
					IsStatic: false,
					Computed: false,
					Body: []*Node{
						{
							Type:     NODE_PROPERTY_DEFINITION,
							Start:    10,
							End:      13,
							IsStatic: false,
							Computed: false,
							Key: &Node{
								Type:  NODE_LITERAL,
								Start: 10,
								End:   13,
								Value: 100,
								Raw:   "1e2",
							},
						},
					},
				},
			},
		},
		SourceType: TYPE_SCRIPT,
	}
	actual, err := GetAst(input, nil, 0)
	if err != nil {
		t.Errorf("Failed to generate ast: %s", err)
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal")
	}
}

func TestCallPropertyThatIsReservedWord(t *testing.T) {
	input := getTestInput("18")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        11,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 0,
				End:   11,
				Object: &Node{
					Type:  NODE_OBJECT_EXPRESSION,
					Start: 1,
					End:   11,
					Name:  "delete",
				},
				Computed: false,
				Optional: false,
			},
		},
	}

	actual, err := GetAst(input, nil, 0)
	if err != nil {
		t.Errorf("Failed to generate ast: %s", err)
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal")
	}
}
