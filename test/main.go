package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DIR_NAME_TEST_SCRIPTS string = "test-scripts"
const DIR_NAME_BENCMARKS string = "benchmark-scripts"
const EXPECTED_TEST_RESULT_RUNNER = "node"
const RUNTIME_BINARY_NAME = "go_js"
const AOC_PUZZLE_LOCATION = "aoc/solution.js"

type failure struct {
	test     string
	reason   string
	expected string
	got      string
}

func main() {
	cases, timeout, err := loadCases(".")
	if err != nil {
		log.Fatalf("failed to load test cases: %s", err)
	}
	fmt.Printf("Running %d tests against NodeJS and GO_JS.\n", len(cases))
	fails := []failure{}
	for index, test := range cases {
		fmt.Printf("%-70s %03d/%d... ", test.label(), index+1, len(cases))
		expected := runNode(test, timeout)
		got := runRuntime(test, timeout)
		if err := compareCase(test, expected, got); err != nil {
			fmt.Println("❌", err)
			fails = append(fails, failure{
				test: test.label(), reason: err.Error(),
				expected: describeExecution(expected), got: describeExecution(got),
			})
		} else {
			fmt.Println("✅")
		}
	}
	if len(fails) > 0 {
		fmt.Println("\nFAILED TESTS")
		for _, result := range fails {
			fmt.Printf("\nTest case: %s\n%s\n-- NodeJS --\n%s\n-- GO_JS --\n%s\n%s\n",
				result.test, result.reason, result.expected, result.got, strings.Repeat("-", 34))
		}
	}
	fmt.Printf("\n%d/%d Tests passed\n\n", len(cases)-len(fails), len(cases))
	failed := len(fails) != 0
	if !runAOC(timeout) {
		failed = true
	}
	if !runBenchmarks(30 * time.Second) {
		failed = true
	}
	if failed {
		os.Exit(1)
	}
}

func runAOC(timeout time.Duration) bool {
	fmt.Println("RUNNING AOC PUZZLE")
	test := testCase{File: AOC_PUZZLE_LOCATION, SourceGoal: "module"}
	expected := runNode(test, timeout)
	got := runRuntime(test, timeout)
	// Preserve the existing puzzle's newline-insensitive object-output check.
	expected.Output = bytes.ReplaceAll(expected.Output, []byte("\n"), nil)
	got.Output = bytes.ReplaceAll(got.Output, []byte("\n"), nil)
	err := compareCase(test, expected, got)
	if err != nil {
		fmt.Println("aoc solution failed ❌", err)
	} else {
		fmt.Println("aoc solution passed ✅")
	}
	fmt.Println("go_js:", describeExecution(got))
	fmt.Println("node: ", describeExecution(expected))
	return err == nil
}

func runBenchmarks(timeout time.Duration) bool {
	fmt.Println("\nRUNNING BENCHMARKS")
	benchmarks, err := os.ReadDir(DIR_NAME_BENCMARKS)
	if err != nil {
		fmt.Println("failed to read benchmark scripts:", err)
		return false
	}
	passed := true
	for _, bench := range benchmarks {
		if bench.IsDir() || filepath.Ext(bench.Name()) != ".js" {
			continue
		}
		test := testCase{File: filepath.Join(DIR_NAME_BENCMARKS, bench.Name()), SourceGoal: "script"}
		expected := runNode(test, timeout)
		got := runRuntime(test, timeout)
		fmt.Printf("\nBenchmark: %s\n-- NodeJS performance --\n%s\n-- GO_JS performance --\n%s\n",
			bench.Name(), describeExecution(expected), describeExecution(got))
		if err := checkOutcome(test, expected); err != nil {
			fmt.Println("NodeJS benchmark failed:", err)
			passed = false
		}
		if err := checkOutcome(test, got); err != nil {
			fmt.Println("GO_JS benchmark failed:", err)
			passed = false
		}
	}
	return passed
}
