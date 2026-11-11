package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func writeFixture(t *testing.T, root, name, contents string) {
	t.Helper()
	filename := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoveryIncludesNewFilesAndSourceGoals(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "cases.json", `{
		"timeoutMs": 1234,
		"sourceGoals": {"environment.js": ["script", "module"], "imports.js": ["module"]},
		"negative": [{"file": "negative-scripts/invalid.js", "sourceGoal": "script", "phase": "parse", "error": "SyntaxError"}]
	}`)
	for _, file := range []string{
		"test-scripts/environment.js", "test-scripts/imports.js", "test-scripts/plain.js",
		"test-scripts/test-module/helper.js", "test-scripts/GC/stress.js",
		"test-scripts/notes.txt", "negative-scripts/invalid.js", "negative-scripts/fixtures/helper.js",
	} {
		writeFixture(t, root, file, "fixture")
	}
	cases, timeout, err := loadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []testCase{
		{File: "test-scripts/environment.js", SourceGoal: "script"},
		{File: "test-scripts/environment.js", SourceGoal: "module"},
		{File: "test-scripts/imports.js", SourceGoal: "module"},
		{File: "test-scripts/plain.js", SourceGoal: "script"},
		{File: "negative-scripts/invalid.js", SourceGoal: "script", Phase: "parse", Error: "SyntaxError"},
	}
	if !reflect.DeepEqual(cases, want) || timeout != 1234*time.Millisecond {
		t.Fatalf("got cases %#v, timeout %s", cases, timeout)
	}
	writeFixture(t, root, "test-scripts/new-feature.js", "fixture")
	cases, _, err = loadCases(root)
	if err != nil || len(cases) != len(want)+1 {
		t.Fatalf("new top-level file was not discovered: %v, %#v", err, cases)
	}
}

func TestInvalidMetadataFailsDiscovery(t *testing.T) {
	for name, metadata := range map[string]string{
		"missing source file": `{"sourceGoals":{"missing.js":["module"]}}`,
		"duplicate goal":      `{"sourceGoals":{"plain.js":["script","script"]}}`,
		"empty goals":         `{"sourceGoals":{"plain.js":[]}}`,
		"invalid goal":        `{"sourceGoals":{"plain.js":["commonjs"]}}`,
		"invalid timeout":     `{"timeoutMs":0}`,
		"missing negative":    `{"negative":[{"file":"negative-scripts/missing.js","sourceGoal":"script","phase":"parse","error":"SyntaxError"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFixture(t, root, "test-scripts/plain.js", "fixture")
			writeFixture(t, root, "cases.json", metadata)
			if _, _, err := loadCases(root); err == nil {
				t.Fatal("invalid metadata was accepted")
			}
		})
	}
}

func TestNegativeTestsRequireMatchingPhaseClassAndOutput(t *testing.T) {
	test := testCase{Phase: "parse", Error: "SyntaxError"}
	expected := execution{Diagnostic: &diagnostic{Outcome: "error", Phase: "parse", Error: "SyntaxError", Message: "Node wording"}}
	for _, scenario := range []struct {
		name string
		got  execution
		pass bool
	}{
		{"matching diagnostic", execution{Diagnostic: &diagnostic{Outcome: "error", Phase: "parse", Error: "SyntaxError", Message: "different engine wording"}}, true},
		{"unexpected success", execution{}, false},
		{"wrong phase", execution{Diagnostic: &diagnostic{Outcome: "error", Phase: "runtime", Error: "SyntaxError"}}, false},
		{"wrong class", execution{Diagnostic: &diagnostic{Outcome: "error", Phase: "parse", Error: "TypeError"}}, false},
		{"compile failure", execution{Diagnostic: &diagnostic{Outcome: "error", Phase: "compile", Error: "SyntaxError"}}, false},
		{"unexpected output", execution{Output: []byte("executed before parsing"), Diagnostic: expected.Diagnostic}, false},
		{"process failure", execution{Err: fmt.Errorf("panic: runtime error")}, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if err := compareCase(test, expected, scenario.got); (err == nil) != scenario.pass {
				t.Fatalf("comparison returned %v", err)
			}
		})
	}
	if compareCase(testCase{}, expected, expected) == nil {
		t.Fatal("matching unexpected errors passed a positive test")
	}
	if compareCase(testCase{}, execution{Output: []byte("a\n")}, execution{Output: []byte("a")}) == nil {
		t.Fatal("stdout comparison discarded a newline")
	}
}

func TestCrashCannotPassAsExpectedError(t *testing.T) {
	report := `{"outcome":"error","phase":"parse","error":"SyntaxError","message":"controlled"}`
	for _, code := range []int{0, 1, 2} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			raw := execute(time.Second, "sh", "-c", `printf '%s' "$1" >&2; exit "$2"`, "sh", report, strconv.Itoa(code))
			result := decodeRuntime(raw)
			err := checkOutcome(testCase{Phase: "parse", Error: "SyntaxError"}, result)
			if (err == nil) != (code == 1) {
				t.Fatalf("exit %d produced %v", code, err)
			}
		})
	}
	result := decodeRuntime(execute(time.Second, "sh", "-c", `echo 'panic: SyntaxError' >&2; exit 1`))
	if result.Err == nil || result.Diagnostic != nil {
		t.Fatal("unstructured crash was treated as an expected JS error")
	}
}

func TestReferenceRequiresCompletionAndPreservesStdout(t *testing.T) {
	for _, output := range []string{"partial output\n", referenceResultMarker + "{", referenceResultMarker + `{}`} {
		if decodeReference(execution{Output: []byte(output)}).Err == nil {
			t.Fatalf("accepted incomplete reference output %q", output)
		}
	}
	data, err := json.Marshal(diagnostic{Outcome: "success"})
	if err != nil {
		t.Fatal(err)
	}
	result := decodeReference(execution{Output: []byte("value\n" + referenceResultMarker + string(data))})
	if result.Err != nil || string(result.Output) != "value\n" {
		t.Fatalf("reference stdout was changed: %#v", result)
	}
}

func TestProcessTimeoutAndOutputLimit(t *testing.T) {
	result := execute(100*time.Millisecond, "sh", "-c", "exec sleep 5")
	if result.Err == nil || !strings.Contains(result.Err.Error(), "timed out") {
		t.Fatalf("hung process was not timed out: %v", result.Err)
	}
	result = execute(time.Second, "sh", "-c", fmt.Sprintf("head -c %d /dev/zero", maxOutputBytes+1))
	if result.Err == nil || !strings.Contains(result.Err.Error(), "exceeded") || len(result.Output) != maxOutputBytes {
		t.Fatalf("output limit failed: error=%v bytes=%d", result.Err, len(result.Output))
	}
	if next := execute(time.Second, "sh", "-c", "printf next"); next.Err != nil || string(next.Output) != "next" {
		t.Fatalf("execution did not continue after failures: %#v", next)
	}
}

func TestNodeAdapterSourceGoalsTimersAndErrorPhases(t *testing.T) {
	if _, err := exec.LookPath(EXPECTED_TEST_RESULT_RUNNER); err != nil {
		t.Skip("Node is supplied by the final Docker test image")
	}
	root := t.TempDir()
	writeFixture(t, root, "dependency.js", "export const present = 1;")
	for _, scenario := range []struct {
		name   string
		source string
		goal   string
		phase  string
		error  string
		output string
	}{
		{
			name: "script globals", source: "var probe = 1; console.log(this === globalThis, Object.hasOwn(globalThis, 'probe'));",
			goal: "script", output: "true true\n",
		},
		{
			name: "module globals", source: "var probe = 1; console.log(this === globalThis, Object.hasOwn(globalThis, 'probe'));",
			goal: "module", output: "false false\n",
		},
		{
			name: "nested timers", source: "console.log('sync'); setTimeout(() => { console.log('timer'); setTimeout(() => console.log('nested'), 0); }, 0);",
			goal: "script", output: "sync\ntimer\nnested\n",
		},
		{name: "parse error", source: "const missing;", goal: "script", phase: "parse", error: "SyntaxError"},
		{name: "link error", source: "import { missing } from './dependency.js';", goal: "module", phase: "link", error: "SyntaxError"},
		{name: "runtime error", source: "null.property;", goal: "script", phase: "runtime", error: "TypeError"},
		{name: "pending async completion", source: "new Promise(() => {});", goal: "script"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			filename := filepath.Join(root, "entry.js")
			writeFixture(t, root, "entry.js", scenario.source)
			test := testCase{File: filename, SourceGoal: scenario.goal, Phase: scenario.phase, Error: scenario.error}
			result := runNode(test, 3*time.Second)
			if scenario.name == "pending async completion" {
				if result.Err == nil || !strings.Contains(result.Err.Error(), "without completing") {
					t.Fatalf("incomplete asynchronous test was accepted: %#v", result)
				}
				return
			}
			if err := checkOutcome(test, result); err != nil {
				t.Fatalf("%v\n%s", err, describeExecution(result))
			}
			if string(result.Output) != scenario.output {
				t.Fatalf("output %q, want %q", result.Output, scenario.output)
			}
		})
	}
}
