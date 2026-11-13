package vm

import (
	"encoding/json"
	"go_js/object"
	"testing"
)

func TestDebugInfoKeepsDuplicateFunctionNamesDistinct(t *testing.T) {
	first := object.NewFunction("ANONYMOYS_FN", 0, nil)
	second := object.NewFunction("ANONYMOYS_FN", 0, nil)

	info := NewDebugInfo()
	info.AddCallable(first, "", false)
	info.AddCallable(second, "", false)
	info.Finalize()

	if len(info.Chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(info.Chunks))
	}
	if info.Chunks[0].ID == "" || info.Chunks[1].ID == "" {
		t.Fatal("chunk ids must be stable and non-empty")
	}
	if info.Chunks[0].ID == info.Chunks[1].ID {
		t.Fatalf("duplicate chunk id %q", info.Chunks[0].ID)
	}
	if info.Chunks[0].Name != "ANONYMOYS_FN" || info.Chunks[1].Name != "ANONYMOYS_FN" {
		t.Fatalf("unexpected chunk names: %#v", info.Chunks)
	}
}

func TestDebugInfoHidesSetupChunk(t *testing.T) {
	setup := object.NewFunction(object.SETUP_FN_NAME, 0, nil)

	info := NewDebugInfo()
	info.AddCallable(setup, "", false)
	info.Finalize()

	if len(info.Chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(info.Chunks))
	}
	chunk := info.Chunks[0]
	if !chunk.Hidden {
		t.Fatal("setup chunk should be hidden")
	}
	if chunk.Kind != "setup" {
		t.Fatalf("expected setup kind, got %q", chunk.Kind)
	}
}

func TestDebugInfoMainOverrideHidesWrapper(t *testing.T) {
	wrapper := object.NewFunction(object.MAIN_FN_NAME, 0, nil)
	entry := object.NewFunction("MODULE /tmp/entry.js", 0, nil)

	info := NewDebugInfo()
	info.AddCallable(wrapper, "", false)
	info.AddCallable(entry, object.MAIN_FN_NAME, true)
	info.Finalize()

	if len(info.Chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(info.Chunks))
	}

	var visible, hidden int
	for _, chunk := range info.Chunks {
		if chunk.Name != object.MAIN_FN_NAME {
			t.Fatalf("unexpected chunk name %q", chunk.Name)
		}
		if chunk.Hidden {
			hidden++
		} else {
			visible++
			if chunk.Kind != "main" {
				t.Fatalf("expected main kind, got %q", chunk.Kind)
			}
		}
	}
	if visible != 1 || hidden != 1 {
		t.Fatalf("expected one visible and one hidden main chunk, got visible=%d hidden=%d", visible, hidden)
	}
}

func TestStructuredOutUsesVersionedDebugInfoAndExecutionPlaceholder(t *testing.T) {
	info := NewDebugInfo()
	info.AddCallable(object.NewFunction("main", 0, nil), "", false)
	info.Finalize()

	out := StructuredOut{
		FormatVersion: StructuredOutputFormatVersion,
		Output:        "hello\n",
		DebugInfo:     info.Legacy,
		DebugChunks:   info.Chunks,
		Execution:     NewExecutionPlaceholder(),
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		FormatVersion int                        `json:"format_version"`
		DebugInfo     map[string]LegacyDebugInfo `json:"debug_info"`
		DebugChunks   []ChunkInfo                `json:"debug_chunks"`
		Execution     struct {
			Status string `json:"status"`
			Frames []any  `json:"frames"`
		} `json:"execution"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.FormatVersion != StructuredOutputFormatVersion {
		t.Fatalf("expected format version %d, got %d", StructuredOutputFormatVersion, decoded.FormatVersion)
	}
	if len(decoded.DebugChunks) != 1 {
		t.Fatalf("expected one debug chunk, got %d", len(decoded.DebugChunks))
	}
	if len(decoded.DebugInfo) != 1 {
		t.Fatalf("expected one legacy debug entry, got %d", len(decoded.DebugInfo))
	}
	if decoded.Execution.Status != "not_requested" {
		t.Fatalf("expected execution placeholder, got %q", decoded.Execution.Status)
	}
	if decoded.Execution.Frames == nil {
		t.Fatal("execution frames must marshal as an empty array, not null")
	}
}
