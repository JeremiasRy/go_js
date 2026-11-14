package value

import "testing"

func TestLocalNamesAtTracksScopesByInstruction(t *testing.T) {
	chunk := NewChunk("main")
	chunk.RegisterLocalScope([]string{"first"})
	chunk.Code = append(chunk.Code, 1)
	chunk.RegisterLocalScope([]string{"first", "second"})
	chunk.Code = append(chunk.Code, 2, 3)

	cases := []struct {
		ip   int
		want []string
	}{
		{0, []string{"first"}},
		{1, []string{"first", "second"}},
		{2, []string{"first", "second"}},
		{3, []string{"first", "second"}},
	}
	for _, test := range cases {
		names := chunk.LocalNamesAt(test.ip)
		if len(names) != len(test.want) {
			t.Fatalf("ip %d: expected %v, got %v", test.ip, test.want, names)
		}
		for i := range names {
			if names[i] != test.want[i] {
				t.Fatalf("ip %d: expected %v, got %v", test.ip, test.want, names)
			}
		}
	}
}

func TestLocalNamesAtReplacesScopeAtSameInstruction(t *testing.T) {
	chunk := NewChunk("main")
	chunk.RegisterLocalScope([]string{"inner"})
	chunk.RegisterLocalScope([]string{"outer"})
	if names := chunk.LocalNamesAt(0); len(names) != 1 || names[0] != "outer" {
		t.Fatalf("expected the latest scope at ip 0, got %v", names)
	}
	if len(chunk.LocalScopes) != 1 {
		t.Fatalf("expected one scope, got %d", len(chunk.LocalScopes))
	}
}
