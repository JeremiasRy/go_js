package parser

import (
	"fmt"
	"testing"
)

// TestCase represents a single test case with input JavaScript and expected AST.
type TestCase struct {
	Name     string
	Input    string
	Expected *Node
}

func RunTests(t *testing.T, cases []TestCase) {
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			input := []byte(tc.Input)
			actual, err := GetAst(input, nil, 0)

			if err != nil {
				t.Fatalf("GetAst returned nil: %s", err.Error())
			}
			if actual == nil {
				t.Fatal("GetAst returned nil")
			}

			if err := compareNodes(actual, tc.Expected, ""); err != nil {
				t.Errorf("AST mismatch: %v", err)
			}
		})
	}
}

func compareNodes(actual, expected *Node, path string) error {
	if actual == nil && expected == nil {
		return nil
	}
	if actual == nil || expected == nil {
		return fmt.Errorf("%s: one node is nil (actual: %v, expected: %v)", path, actual, expected)
	}

	if actual.Type != expected.Type {
		return fmt.Errorf("%s.Type: got %v, want %v", path, actual.Type, expected.Type)
	}
	if actual.Start != expected.Start {
		return fmt.Errorf("%s.Start: got %d, want %d", path, actual.Start, expected.Start)
	}
	if actual.End != expected.End {
		return fmt.Errorf("%s.End: got %d, want %d", path, actual.End, expected.End)
	}

	switch actual.Type {
	case NODE_PROGRAM:
		if actual.SourceType != expected.SourceType {
			return fmt.Errorf("%s.SourceType: got %v, want %v", path, actual.SourceType, expected.SourceType)
		}
		if len(actual.Body) != len(expected.Body) {
			return fmt.Errorf("%s.Body: length mismatch, got %d, want %d", path, len(actual.Body), len(expected.Body))
		}
		for i := range actual.Body {
			if err := compareNodes(actual.Body[i], expected.Body[i], fmt.Sprintf("%s.Body[%d]", path, i)); err != nil {
				return err
			}
		}
	case NODE_CLASS_DECLARATION:
		if err := compareNodes(actual.Id, expected.Id, path+".Id"); err != nil {
			return err
		}
		if err := compareNodes(actual.SuperClass, expected.SuperClass, path+".SuperClass"); err != nil {
			return err
		}
		if err := compareNodes(actual.ClassBody, expected.ClassBody, path+".ClassBody"); err != nil {
			return err
		}
	case NODE_CLASS_BODY:
		if len(actual.Body) != len(expected.Body) {
			return fmt.Errorf("%s.Body: length mismatch, got %d, want %d", path, len(actual.Body), len(expected.Body))
		}
		for i := range actual.Body {
			if err := compareNodes(actual.Body[i], expected.Body[i], fmt.Sprintf("%s.Body[%d]", path, i)); err != nil {
				return err
			}
		}
	case NODE_PROPERTY_DEFINITION:
		if actual.IsStatic != expected.IsStatic {
			return fmt.Errorf("%s.IsStatic: got %v, want %v", path, actual.IsStatic, expected.IsStatic)
		}
		if actual.Computed != expected.Computed {
			return fmt.Errorf("%s.Computed: got %v, want %v", path, actual.Computed, expected.Computed)
		}
		if err := compareNodes(actual.Key, expected.Key, path+".Key"); err != nil {
			return err
		}
		if err := compareNodes(actual.Value.(*Node), expected.Value.(*Node), path+".Value"); err != nil {
			return err
		}
	case NODE_IDENTIFIER:
		if actual.Name != expected.Name {
			return fmt.Errorf("%s.Name: got %q, want %q", path, actual.Name, expected.Name)
		}
	}

	return nil
}

func TestParser(t *testing.T) {
	// Example test case based on Acorn's "class C { aaa }".
	cases := []TestCase{
		{
			Name:  "Class Declaration",
			Input: "class C { aaa }", // TODO: read from file, ./test_cripts/test_3 etc...
			Expected: &Node{
				Type:       NODE_PROGRAM,
				Start:      0,
				End:        15,
				SourceType: TYPE_SCRIPT,
				Body: []*Node{
					{
						Type:  NODE_CLASS_DECLARATION,
						Start: 0,
						End:   15,
						Id: &Node{
							Type:  NODE_IDENTIFIER,
							Start: 6,
							End:   7,
							Name:  "C",
						},
						SuperClass: nil,
						ClassBody: &Node{
							Type:  NODE_CLASS_BODY,
							Start: 8,
							End:   15,
							Body: []*Node{
								{
									Type:     NODE_PROPERTY_DEFINITION,
									Start:    10,
									End:      13,
									IsStatic: false,
									Computed: false,
									Key: &Node{
										Type:  NODE_IDENTIFIER,
										Start: 10,
										End:   13,
										Name:  "aaa",
									},
									Value: nil,
								},
							},
						},
					},
				},
			},
		},
		// Add more test cases here.
	}

	RunTests(t, cases)
}
