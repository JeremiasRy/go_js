package parser

import (
	"strings"
	"testing"
)

type Regexp struct {
	pattern string
}

func (r *Regexp) Match(b []byte) bool {
	return strings.Contains(string(b), r.pattern) // Simplified for testing
}

func TestReadString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		quote    rune
		expected string
		errMsg   string
	}{
		{
			name:     "double quoted string",
			input:    `"hello"`,
			quote:    '"',
			expected: "hello",
			errMsg:   "",
		},
		{
			name:     "single quoted string",
			input:    `'world'`,
			quote:    '\'',
			expected: "world",
			errMsg:   "",
		},
		{
			name:     "escaped newline",
			input:    `"escaped\n"`,
			quote:    '"',
			expected: "escaped\n",
			errMsg:   "",
		},
		{
			name:     "incomplete string",
			input:    `"incomplete`,
			quote:    '"',
			expected: "",
			errMsg:   "Unterminated string constant",
		},
		{
			name:     "unicode escape",
			input:    `"unicode\u0041"`,
			quote:    '"',
			expected: "unicodeA",
			errMsg:   "",
		},
		{
			name:     "invalid unicode escape",
			input:    `"unicode\u00"`,
			quote:    '"',
			expected: "",
			errMsg:   "Unterminated string constant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize parser with test input
			p := &Parser{
				input: []byte(tt.input),
				pos:   0,
				options: Options{
					ecmaVersion: 6, // Assume ECMAScript 6 for testing
					Locations:   false,
				},
				Type:      tokenTypes[TOKEN_STRING],
				CurLine:   1,
				LineStart: 0,
			}

			// Run the readString method
			err := p.readString(tt.quote)

			// Check for expected error
			if tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error %q, got %v", tt.errMsg, err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify the output value
			got, ok := p.Value.([]byte)
			if !ok {
				t.Errorf("expected Value to be []byte, got %T", p.Value)
				return
			}
			if string(got) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(got))
			}
		})
	}
}
