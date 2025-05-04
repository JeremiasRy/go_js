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
	aJson, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return false
	}
	bJson, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return false
	}
	areEqual := reflect.DeepEqual(aJson, bJson)
	if !areEqual {
		println("### EXPECTED ###")
		println(string(bJson))
		println("### GOT ###")
		println(string(aJson))
	}

	return areEqual
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

func TestClassPropertyUnicode(t *testing.T) {
	input := getTestInput("4")
	expected := &Node{
		Type:  NODE_PROGRAM,
		Start: 0,
		End:   18,
		Body: []*Node{
			{
				Type:  NODE_CLASS_DECLARATION,
				Start: 0,
				End:   18,
				Identifier: &Node{
					Type:  NODE_IDENTIFIER,
					Start: 6,
					End:   7,
					Name:  "C",
				},
				BodyNode: &Node{
					Type:     NODE_CLASS_BODY,
					Start:    8,
					End:      18,
					IsStatic: false,
					Computed: false,
					Body: []*Node{
						{
							Type:     NODE_PROPERTY_DEFINITION,
							Start:    10,
							End:      16,
							IsStatic: false,
							Computed: false,
							Key: &Node{
								Type:  NODE_IDENTIFIER,
								Start: 10,
								End:   16,
								Name:  "A",
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

func TestClassPrivateMethod(t *testing.T) {
	input := getTestInput("5")
	expected := &Node{
		Type:  NODE_PROGRAM,
		Start: 0,
		End:   22,
		Body: []*Node{
			{
				Type:  NODE_CLASS_DECLARATION,
				Start: 0,
				End:   22,
				Identifier: &Node{
					Type:  NODE_IDENTIFIER,
					Start: 6,
					End:   7,
					Name:  "C",
				},
				BodyNode: &Node{
					Type:     NODE_CLASS_BODY,
					Start:    8,
					End:      22,
					IsStatic: false,
					Computed: false,
					Body: []*Node{
						{
							Type:     NODE_METHOD_DEFINITION,
							Start:    10,
							End:      20,
							IsStatic: false,
							Computed: false,
							Key: &Node{
								Type:  NODE_PRIVATE_IDENTIFIER,
								Start: 10,
								End:   14,
								Name:  "aaa",
							},
							Kind: KIND_PROPERTY_METHOD,
							Value: &Node{
								Type:         NODE_FUNCTION_EXPRESSION,
								Start:        14,
								End:          20,
								IsExpression: false,
								IsGenerator:  false,
								IsAsync:      false,
								BodyNode: &Node{
									Type:  NODE_BLOCK_STATEMENT,
									Start: 17,
									End:   20,
								},
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

func TestAsyncIterator(t *testing.T) {
	input := getTestInput("6")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        43,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_FUNCTION_DECLARATION,
				Start: 0,
				End:   43,
				Identifier: &Node{
					Start: 15,
					End:   16,
					Name:  "f",
				},
				IsAsync: true,
				BodyNode: &Node{
					Type:  NODE_BLOCK_STATEMENT,
					Start: 19,
					End:   43,
					Body: []*Node{
						{
							Type:  NODE_FOR_OF_STATEMENT,
							Start: 21,
							End:   41,
							Await: true,
							Left: &Node{
								Type:  NODE_IDENTIFIER,
								Start: 32,
								End:   33,
								Name:  "x",
							},
							Right: &Node{
								Type:  NODE_IDENTIFIER,
								Start: 37,
								End:   39,
								Name:  "xs",
							},
							BodyNode: &Node{
								Type:  NODE_EMPTY_STATEMENT,
								Start: 40,
								End:   41,
							},
						},
					},
				},
			},
		},
	}

	actual, err := GetAst(input, nil, 0)

	if err != nil {
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestArithmetic(t *testing.T) {
	input := getTestInput("7")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        36,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_VARIABLE_DECLARATION,
				Start: 0,
				End:   36,
				Kind:  KIND_DECLARATION_CONST,
				Declarations: []*Node{
					{
						Type:  NODE_VARIABLE_DECLARATOR,
						Start: 6,
						End:   36,
						Identifier: &Node{
							Type:  NODE_IDENTIFIER,
							Start: 6,
							End:   7,
							Name:  "x",
						},
						Initializer: &Node{
							Type:           NODE_BINARY_EXPRESSION,
							Start:          10,
							End:            36,
							BinaryOperator: "+",
							Left: &Node{
								Type:           NODE_BINARY_EXPRESSION,
								Start:          10,
								End:            31,
								BinaryOperator: "*",
								Left: &Node{
									Type:  NODE_LITERAL,
									Start: 10,
									End:   11,
									Value: 1,
									Raw:   "1",
								},
								Right: &Node{
									Type:           NODE_BINARY_EXPRESSION,
									Start:          15,
									End:            30,
									BinaryOperator: "-",
									Left: &Node{
										Type:  NODE_LITERAL,
										Start: 15,
										End:   16,
										Value: 1,
										Raw:   "1",
									},
									Right: &Node{
										Type:           NODE_BINARY_EXPRESSION,
										Start:          19,
										End:            30,
										BinaryOperator: "/",
										Left: &Node{
											Type:  NODE_LITERAL,
											Start: 19,
											End:   20,
											Value: 4,
											Raw:   "4",
										},
										Right: &Node{
											Type:           NODE_BINARY_EXPRESSION,
											Start:          24,
											End:            29,
											BinaryOperator: "+",
											Left: &Node{
												Type:  NODE_LITERAL,
												Start: 24,
												End:   25,
												Value: 1,
												Raw:   "1",
											},
											Right: &Node{
												Type:  NODE_LITERAL,
												Start: 28,
												End:   29,
												Value: 3,
												Raw:   "3",
											},
										},
									},
								},
							},
							Right: &Node{
								Type:  NODE_LITERAL,
								Start: 34,
								End:   36,
								Value: 12,
								Raw:   "12",
							},
						},
					},
				},
			},
		},
	}

	actual, err := GetAst(input, nil, 0)

	if err != nil {
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestAssignmentOperators(t *testing.T) {
	input := getTestInput("8")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        117,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_VARIABLE_DECLARATION,
				Start: 0,
				End:   11,
				Kind:  KIND_DECLARATION_LET,
				Declarations: []*Node{
					{
						Type:  NODE_VARIABLE_DECLARATOR,
						Start: 4,
						End:   11,
						Identifier: &Node{
							Type:  NODE_IDENTIFIER,
							Start: 4,
							End:   5,
							Name:  "a",
						},
						Initializer: &Node{
							Type:  NODE_LITERAL,
							Start: 8,
							End:   11,
							Value: 100,
							Raw:   "100",
						},
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 12,
				End:   18,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              12,
					End:                18,
					AssignmentOperator: "-=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 12,
						End:   13,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 17,
						End:   18,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 19,
				End:   25,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              19,
					End:                25,
					AssignmentOperator: "*=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 19,
						End:   20,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 24,
						End:   25,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 26,
				End:   32,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              26,
					End:                32,
					AssignmentOperator: "/=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 26,
						End:   27,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 31,
						End:   32,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 33,
				End:   39,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              33,
					End:                39,
					AssignmentOperator: "&=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 33,
						End:   34,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 38,
						End:   39,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 40,
				End:   46,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              40,
					End:                46,
					AssignmentOperator: "|=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 40,
						End:   41,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 45,
						End:   46,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 47,
				End:   53,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              47,
					End:                53,
					AssignmentOperator: "^=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 47,
						End:   48,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 52,
						End:   53,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 54,
				End:   61,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              54,
					End:                61,
					AssignmentOperator: "**=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 54,
						End:   55,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 60,
						End:   61,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 62,
				End:   68,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              62,
					End:                68,
					AssignmentOperator: "%=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 62,
						End:   63,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 67,
						End:   68,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 69,
				End:   76,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              69,
					End:                76,
					AssignmentOperator: "<<=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 69,
						End:   70,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 75,
						End:   76,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 77,
				End:   84,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              77,
					End:                84,
					AssignmentOperator: ">>=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 77,
						End:   78,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 83,
						End:   84,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 85,
				End:   93,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              85,
					End:                93,
					AssignmentOperator: ">>>=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 85,
						End:   86,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 92,
						End:   93,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 94,
				End:   101,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              94,
					End:                101,
					AssignmentOperator: "&&=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 94,
						End:   95,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 100,
						End:   101,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 102,
				End:   109,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              102,
					End:                109,
					AssignmentOperator: "||=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 102,
						End:   103,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 108,
						End:   109,
						Value: 2,
						Raw:   "2",
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 110,
				End:   117,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              110,
					End:                117,
					AssignmentOperator: "??=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 110,
						End:   111,
						Name:  "a",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 116,
						End:   117,
						Value: 2,
						Raw:   "2",
					},
				},
			},
		},
	}

	actual, err := GetAst(input, nil, 0)

	if err != nil {
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestAsyncIteratorVar(t *testing.T) {
	input := getTestInput("9")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        47,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_FUNCTION_DECLARATION,
				Start: 0,
				End:   47,
				Identifier: &Node{
					Type:  NODE_IDENTIFIER,
					Start: 15,
					End:   16,
					Name:  "f",
				},
				IsAsync: true,
				BodyNode: &Node{
					Type:  NODE_BLOCK_STATEMENT,
					Start: 19,
					End:   47,
					Body: []*Node{
						{
							Type:  NODE_FOR_OF_STATEMENT,
							Start: 21,
							End:   45,
							Await: true,
							Left: &Node{
								Type:  NODE_VARIABLE_DECLARATION,
								Start: 32,
								End:   37,
								Kind:  KIND_DECLARATION_VAR,
								Declarations: []*Node{
									{
										Type:  NODE_VARIABLE_DECLARATOR,
										Start: 36,
										End:   37,
										Identifier: &Node{
											Type:  NODE_IDENTIFIER,
											Start: 36,
											End:   37,
											Name:  "x",
										},
										Initializer: nil,
									},
								},
							},
							Right: &Node{
								Type:  NODE_IDENTIFIER,
								Start: 41,
								End:   43,
								Name:  "xs",
							},
							BodyNode: &Node{
								Type:  NODE_EMPTY_STATEMENT,
								Start: 44,
								End:   45,
							},
						},
					},
				},
			},
		},
	}

	actual, err := GetAst(input, nil, 0)

	if err != nil {
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestAsyncITeratorNewLine(t *testing.T) {
	input := getTestInput("10")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        52,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_FUNCTION_DECLARATION,
				Start: 0,
				End:   52,
				Identifier: &Node{
					Type:  NODE_IDENTIFIER,
					Start: 15,
					End:   16,
					Name:  "f",
				},
				IsAsync: true,
				BodyNode: &Node{
					Type:  NODE_BLOCK_STATEMENT,
					Start: 19,
					End:   52,
					Body: []*Node{
						{
							Type:  NODE_FOR_OF_STATEMENT,
							Start: 21,
							End:   50,
							Await: true,
							Left: &Node{
								Type:  NODE_VARIABLE_DECLARATION,
								Start: 37,
								End:   42,
								Kind:  KIND_DECLARATION_LET,
								Declarations: []*Node{
									{
										Type:  NODE_VARIABLE_DECLARATOR,
										Start: 41,
										End:   42,
										Identifier: &Node{
											Type:  NODE_IDENTIFIER,
											Start: 41,
											End:   42,
											Name:  "x",
										},
										Initializer: nil,
									},
								},
							},
							Right: &Node{
								Type:  NODE_IDENTIFIER,
								Start: 46,
								End:   48,
								Name:  "xs",
							},
							BodyNode: &Node{
								Type:  NODE_EMPTY_STATEMENT,
								Start: 49,
								End:   50,
							},
						},
					},
				},
			},
		},
	}

	actual, err := GetAst(input, nil, 0)

	if err != nil {
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestGeneratorAwaitKeyword(t *testing.T) {
	input := getTestInput("11")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        52,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 0,
				End:   52,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              0,
					End:                52,
					AssignmentOperator: "=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 0,
						End:   1,
						Name:  "f",
					},
					Right: &Node{
						Type:        NODE_FUNCTION_EXPRESSION,
						Start:       4,
						End:         52,
						Identifier:  nil,
						IsGenerator: true,
						IsAsync:     true,
						BodyNode: &Node{
							Type:  NODE_BLOCK_STATEMENT,
							Start: 23,
							End:   52,
							Body: []*Node{
								{
									Type:  NODE_EXPRESSION_STATEMENT,
									Start: 29,
									End:   37,
									Expression: &Node{
										Type:  NODE_AWAIT_EXPRESSION,
										Start: 29,
										End:   36,
										Argument: &Node{
											Type:  NODE_IDENTIFIER,
											Start: 35,
											End:   36,
											Name:  "a",
										},
									},
								},
								{
									Type:  NODE_EXPRESSION_STATEMENT,
									Start: 42,
									End:   50,
									Expression: &Node{
										Type:     NODE_YIELD_EXPRESSION,
										Start:    42,
										End:      49,
										Delegate: false,
										Argument: &Node{
											Type:  NODE_IDENTIFIER,
											Start: 48,
											End:   49,
											Name:  "b",
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
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestClasStaticAsyncGenerator(t *testing.T) {
	input := getTestInput("12")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        51,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_CLASS_DECLARATION,
				Start: 0,
				End:   51,
				Identifier: &Node{
					Type:  NODE_IDENTIFIER,
					Start: 6,
					End:   7,
					Name:  "A",
				},
				SuperClass: nil,
				BodyNode: &Node{
					Type:  NODE_CLASS_BODY,
					Start: 8,
					End:   51,
					Body: []*Node{
						{
							Type:     NODE_METHOD_DEFINITION,
							Start:    10,
							End:      49,
							IsStatic: true,
							Computed: false,
							Key: &Node{
								Type:  NODE_IDENTIFIER,
								Start: 24,
								End:   25,
								Name:  "f",
							},
							Kind: KIND_PROPERTY_METHOD,
							Value: &Node{
								Type:        NODE_FUNCTION_EXPRESSION,
								Start:       25,
								End:         49,
								Identifier:  nil,
								IsGenerator: true,
								IsAsync:     true,
								BodyNode: &Node{
									Type:  NODE_BLOCK_STATEMENT,
									Start: 28,
									End:   49,
									Body: []*Node{
										{
											Type:  NODE_EXPRESSION_STATEMENT,
											Start: 30,
											End:   38,
											Expression: &Node{
												Type:  NODE_AWAIT_EXPRESSION,
												Start: 30,
												End:   37,
												Argument: &Node{
													Type:  NODE_IDENTIFIER,
													Start: 36,
													End:   37,
													Name:  "a",
												},
											},
										},
										{
											Type:  NODE_EXPRESSION_STATEMENT,
											Start: 39,
											End:   47,
											Expression: &Node{
												Type:     NODE_YIELD_EXPRESSION,
												Start:    39,
												End:      46,
												Delegate: false,
												Argument: &Node{
													Type:  NODE_IDENTIFIER,
													Start: 45,
													End:   46,
													Name:  "b",
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
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestClasAsyncGenerator(t *testing.T) {
	input := getTestInput("13")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        44,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_CLASS_DECLARATION,
				Start: 0,
				End:   44,
				Identifier: &Node{
					Type:  NODE_IDENTIFIER,
					Start: 6,
					End:   7,
					Name:  "A",
				},
				SuperClass: nil,
				BodyNode: &Node{
					Type:  NODE_CLASS_BODY,
					Start: 8,
					End:   44,
					Body: []*Node{
						{
							Type:     NODE_METHOD_DEFINITION,
							Start:    10,
							End:      42,
							IsStatic: false,
							Computed: false,
							Key: &Node{
								Type:  NODE_IDENTIFIER,
								Start: 17,
								End:   18,
								Name:  "f",
							},
							Kind: KIND_PROPERTY_METHOD,
							Value: &Node{
								Type:        NODE_FUNCTION_EXPRESSION,
								Start:       18,
								End:         42,
								Identifier:  nil,
								IsGenerator: true,
								IsAsync:     true,
								BodyNode: &Node{
									Type:  NODE_BLOCK_STATEMENT,
									Start: 21,
									End:   42,
									Body: []*Node{
										{
											Type:  NODE_EXPRESSION_STATEMENT,
											Start: 23,
											End:   31,
											Expression: &Node{
												Type:  NODE_AWAIT_EXPRESSION,
												Start: 23,
												End:   30,
												Argument: &Node{
													Type:  NODE_IDENTIFIER,
													Start: 29,
													End:   30,
													Name:  "a",
												},
											},
										},
										{
											Type:  NODE_EXPRESSION_STATEMENT,
											Start: 32,
											End:   40,
											Expression: &Node{
												Type:     NODE_YIELD_EXPRESSION,
												Start:    32,
												End:      39,
												Delegate: false,
												Argument: &Node{
													Type:  NODE_IDENTIFIER,
													Start: 38,
													End:   39,
													Name:  "b",
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
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestAsyncGeratorObjAssignment(t *testing.T) {
	input := getTestInput("14")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        42,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 0,
				End:   42,
				Expression: &Node{
					Type:               NODE_ASSIGNMENT_EXPRESSION,
					Start:              0,
					End:                42,
					AssignmentOperator: "=",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 0,
						End:   3,
						Name:  "obj",
					},
					Right: &Node{
						Type:  NODE_OBJECT_EXPRESSION,
						Start: 6,
						End:   42,
						Properties: []*Node{
							{
								Type:      NODE_PROPERTY,
								Start:     8,
								End:       40,
								IsMethod:  true,
								Shorthand: false,
								Computed:  false,
								Key: &Node{
									Type:  NODE_IDENTIFIER,
									Start: 15,
									End:   16,
									Name:  "f",
								},
								Kind: KIND_PROPERTY_INIT,
								Value: &Node{
									Type:        NODE_FUNCTION_EXPRESSION,
									Start:       16,
									End:         40,
									Identifier:  nil,
									IsGenerator: true,
									IsAsync:     true,
									BodyNode: &Node{
										Type:  NODE_BLOCK_STATEMENT,
										Start: 19,
										End:   40,
										Body: []*Node{
											{
												Type:  NODE_EXPRESSION_STATEMENT,
												Start: 21,
												End:   29,
												Expression: &Node{
													Type:  NODE_AWAIT_EXPRESSION,
													Start: 21,
													End:   28,
													Argument: &Node{
														Type:  NODE_IDENTIFIER,
														Start: 27,
														End:   28,
														Name:  "a",
													},
												},
											},
											{
												Type:  NODE_EXPRESSION_STATEMENT,
												Start: 30,
												End:   38,
												Expression: &Node{
													Type:     NODE_YIELD_EXPRESSION,
													Start:    30,
													End:      37,
													Delegate: false,
													Argument: &Node{
														Type:  NODE_IDENTIFIER,
														Start: 36,
														End:   37,
														Name:  "b",
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
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestOrCoalesce(t *testing.T) {
	input := getTestInput("15")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        14,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 0,
				End:   14,
				Expression: &Node{
					Type:           NODE_LOGICAL_EXPRESSION,
					Start:          0,
					End:            14,
					BinaryOperator: "??",
					Left: &Node{
						Type:           NODE_BINARY_EXPRESSION,
						Start:          0,
						End:            5,
						BinaryOperator: "|",
						Left: &Node{
							Type:  NODE_IDENTIFIER,
							Start: 0,
							End:   1,
							Name:  "a",
						},
						Right: &Node{
							Type:  NODE_IDENTIFIER,
							Start: 4,
							End:   5,
							Name:  "b",
						},
					},
					Right: &Node{
						Type:           NODE_BINARY_EXPRESSION,
						Start:          9,
						End:            14,
						BinaryOperator: "|",
						Left: &Node{
							Type:  NODE_IDENTIFIER,
							Start: 9,
							End:   10,
							Name:  "c",
						},
						Right: &Node{
							Type:  NODE_IDENTIFIER,
							Start: 13,
							End:   14,
							Name:  "d",
						},
					},
				},
			},
		},
	}
	actual, err := GetAst(input, nil, 0)

	if err != nil {
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestForLoop(t *testing.T) {
	input := getTestInput("16")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        51,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_FOR_STATEMENT,
				Start: 0,
				End:   51,
				Initializer: &Node{
					Type:  NODE_VARIABLE_DECLARATION,
					Start: 5,
					End:   14,
					Kind:  KIND_DECLARATION_LET,
					Declarations: []*Node{
						{
							Type:  NODE_VARIABLE_DECLARATOR,
							Start: 9,
							End:   14,
							Identifier: &Node{
								Type:  NODE_IDENTIFIER,
								Start: 9,
								End:   10,
								Name:  "i",
							},
							Initializer: &Node{
								Type:  NODE_LITERAL,
								Start: 13,
								End:   14,
								Value: 0,
								Raw:   "0",
							},
						},
					},
				},
				Test: &Node{
					Type:           NODE_BINARY_EXPRESSION,
					Start:          16,
					End:            22,
					BinaryOperator: "<",
					Left: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 16,
						End:   17,
						Name:  "i",
					},
					Right: &Node{
						Type:  NODE_LITERAL,
						Start: 20,
						End:   22,
						Value: 10,
						Raw:   "10",
					},
				},
				Update: &Node{
					Type:           NODE_UPDATE_EXPRESSION,
					Start:          24,
					End:            27,
					UpdateOperator: "++",
					Prefix:         false,
					Argument: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 24,
						End:   25,
						Name:  "i",
					},
				},
				BodyNode: &Node{
					Type:  NODE_BLOCK_STATEMENT,
					Start: 29,
					End:   51,
					Body: []*Node{
						{
							Type:  NODE_EXPRESSION_STATEMENT,
							Start: 35,
							End:   49,
							Expression: &Node{
								Type:     NODE_CALL_EXPRESSION,
								Start:    35,
								End:      49,
								Optional: false,
								Callee: &Node{
									Type:     NODE_MEMBER_EXPRESSION,
									Start:    35,
									End:      46,
									Computed: false,
									Optional: false,
									Object: &Node{
										Type:  NODE_IDENTIFIER,
										Start: 35,
										End:   42,
										Name:  "console",
									},
									Property: &Node{
										Type:  NODE_IDENTIFIER,
										Start: 43,
										End:   46,
										Name:  "log",
									},
								},
								Arguments: []*Node{
									{
										Type:  NODE_IDENTIFIER,
										Start: 47,
										End:   48,
										Name:  "i",
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
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestSwitchStatement(t *testing.T) {
	input := getTestInput("17")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        196,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_SWITCH_STATEMENT,
				Start: 0,
				End:   196,
				Discriminant: &Node{
					Type:  NODE_LITERAL,
					Start: 8,
					End:   9,
					Value: 1,
					Raw:   "1",
				},
				Cases: []*Node{
					{
						Type:  NODE_SWITCH_CASE,
						Start: 17,
						End:   74,
						Test: &Node{
							Type:  NODE_LITERAL,
							Start: 22,
							End:   23,
							Value: 1,
							Raw:   "1",
						},
						ConsequentSlice: []*Node{
							{
								Type:  NODE_BLOCK_STATEMENT,
								Start: 25,
								End:   74,
								Body: []*Node{
									{
										Type:  NODE_EXPRESSION_STATEMENT,
										Start: 35,
										End:   53,
										Expression: &Node{
											Type:     NODE_CALL_EXPRESSION,
											Start:    35,
											End:      53,
											Optional: false,
											Callee: &Node{
												Type:     NODE_MEMBER_EXPRESSION,
												Start:    35,
												End:      46,
												Computed: false,
												Optional: false,
												Object: &Node{
													Type:  NODE_IDENTIFIER,
													Start: 35,
													End:   42,
													Name:  "console",
												},
												Property: &Node{
													Type:  NODE_IDENTIFIER,
													Start: 43,
													End:   46,
													Name:  "log",
												},
											},
											Arguments: []*Node{
												{
													Type:  NODE_LITERAL,
													Start: 47,
													End:   52,
													Value: "one",
													Raw:   "\"one\"",
												},
											},
										},
									},
									{
										Type:  NODE_BREAK_STATEMENT,
										Start: 62,
										End:   68,
										Label: nil,
									},
								},
							},
						},
					},
					{
						Type:  NODE_SWITCH_CASE,
						Start: 79,
						End:   86,
						Test: &Node{
							Type:  NODE_LITERAL,
							Start: 84,
							End:   85,
							Value: 2,
							Raw:   "2",
						},
					},
					{
						Type:  NODE_SWITCH_CASE,
						Start: 91,
						End:   142,
						Test: &Node{
							Type:  NODE_LITERAL,
							Start: 96,
							End:   97,
							Value: 3,
							Raw:   "3",
						},
						ConsequentSlice: []*Node{
							{
								Type:  NODE_BLOCK_STATEMENT,
								Start: 99,
								End:   142,
								Body: []*Node{
									{
										Type:  NODE_EXPRESSION_STATEMENT,
										Start: 109,
										End:   136,
										Expression: &Node{
											Type:     NODE_CALL_EXPRESSION,
											Start:    109,
											End:      136,
											Optional: false,
											Callee: &Node{
												Type:     NODE_MEMBER_EXPRESSION,
												Start:    109,
												End:      120,
												Computed: false,
												Optional: false,
												Object: &Node{
													Type:  NODE_IDENTIFIER,
													Start: 109,
													End:   116,
													Name:  "console",
												},
												Property: &Node{
													Type:  NODE_IDENTIFIER,
													Start: 117,
													End:   120,
													Name:  "log",
												},
											},
											Arguments: []*Node{
												{
													Type:  NODE_LITERAL,
													Start: 121,
													End:   135,
													Value: "two or three",
													Raw:   "\"two or three\"",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Type:  NODE_SWITCH_CASE,
						Start: 147,
						End:   194,
						Test:  nil,
						ConsequentSlice: []*Node{
							{
								Type:  NODE_BLOCK_STATEMENT,
								Start: 156,
								End:   194,
								Body: []*Node{
									{
										Type:  NODE_EXPRESSION_STATEMENT,
										Start: 166,
										End:   188,
										Expression: &Node{
											Type:     NODE_CALL_EXPRESSION,
											Start:    166,
											End:      188,
											Optional: false,
											Callee: &Node{
												Type:     NODE_MEMBER_EXPRESSION,
												Start:    166,
												End:      177,
												Computed: false,
												Optional: false,
												Object: &Node{
													Type:  NODE_IDENTIFIER,
													Start: 166,
													End:   173,
													Name:  "console",
												},
												Property: &Node{
													Type:  NODE_IDENTIFIER,
													Start: 174,
													End:   177,
													Name:  "log",
												},
											},
											Arguments: []*Node{
												{
													Type:  NODE_LITERAL,
													Start: 178,
													End:   187,
													Value: "default",
													Raw:   "\"default\"",
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
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
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
				Expression: &Node{
					Type:  NODE_MEMBER_EXPRESSION,
					Start: 0,
					End:   11,
					Object: &Node{
						Type:  NODE_OBJECT_EXPRESSION,
						Start: 1,
						End:   3,
					},
					Property: &Node{
						Type:  NODE_IDENTIFIER,
						Start: 5,
						End:   11,
						Name:  "delete",
					},
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

func TestDecimalNumberToSTring(t *testing.T) {
	input := getTestInput("19")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        13,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 0,
				End:   13,
				Expression: &Node{
					Type:     NODE_CALL_EXPRESSION,
					Start:    0,
					End:      13,
					Optional: false,
					Callee: &Node{
						Type:     NODE_MEMBER_EXPRESSION,
						Start:    0,
						End:      11,
						Computed: false,
						Optional: false,
						Object: &Node{
							Type:  NODE_LITERAL,
							Start: 0,
							End:   2,
							Value: 0,
							Raw:   "0.",
						},
						Property: &Node{
							Type:  NODE_IDENTIFIER,
							Start: 3,
							End:   11,
							Name:  "toString",
						},
					},
					Arguments: []*Node{},
				},
			},
		},
	}
	actual, err := GetAst(input, nil, 0)

	if err != nil {
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}

func TestDeviousFunctionDeclarationCallAdd(t *testing.T) {
	input := getTestInput("20")
	expected := &Node{
		Type:       NODE_PROGRAM,
		Start:      0,
		End:        32,
		SourceType: TYPE_SCRIPT,
		Body: []*Node{
			{
				Type:  NODE_FUNCTION_DECLARATION,
				Start: 0,
				End:   26,
				Identifier: &Node{
					Type:  NODE_IDENTIFIER,
					Start: 9,
					End:   10,
					Name:  "f",
				},
				IsExpression: false,
				IsGenerator:  false,
				IsAsync:      false,
				BodyNode: &Node{
					Type:  NODE_BLOCK_STATEMENT,
					Start: 13,
					End:   26,
					Body: []*Node{
						{
							Type:  NODE_RETURN_STATEMENT,
							Start: 15,
							End:   24,
							Argument: &Node{
								Type:          NODE_UNARY_EXPRESSION,
								Start:         22,
								End:           24,
								UnaryOperator: "!",
								Prefix:        true,
								Argument: &Node{
									Type:  NODE_LITERAL,
									Start: 23,
									End:   24,
									Value: 1,
									Raw:   "1",
								},
							},
						},
					},
				},
			},
			{
				Type:  NODE_EXPRESSION_STATEMENT,
				Start: 27,
				End:   32,
				Expression: &Node{
					Type:          NODE_UNARY_EXPRESSION,
					Start:         27,
					End:           32,
					UnaryOperator: "+",
					Prefix:        true,
					Argument: &Node{
						Type:     NODE_CALL_EXPRESSION,
						Start:    29,
						End:      32,
						Optional: false,
						Callee: &Node{
							Type:  NODE_IDENTIFIER,
							Start: 29,
							End:   30,
							Name:  "f",
						},
						Arguments: []*Node{},
					},
				},
			},
		},
	}
	actual, err := GetAst(input, nil, 0)

	if err != nil {
		t.Errorf("Failed to get AST: %s", err.Error())
	}

	if !areNodesEqual(actual, expected) {
		t.Errorf("Nodes are not equal.")
	}
}
