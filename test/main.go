package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

const DIR_NAME_TEST_SCRIPTS string = "test-scripts"
const DIR_NAME_BENCMARKS string = "benchmark-scripts"

const PREFIX_TEST_SCIPT string = "mission"

const EXPECTED_TEST_RESULT_RUNNER = "node"
const RUNTIME_BINARY_NAME = "go_js"

func main() {
	benchmarks, err := os.ReadDir(DIR_NAME_BENCMARKS)

	if err != nil {
		log.Fatalf("failed to read benchmark scripts %s", err)
	}

	scripts, err := os.ReadDir(DIR_NAME_TEST_SCRIPTS)

	if err != nil {
		log.Fatalf("failed to read test scripts %s", err)
	}

	expected := map[string][]byte{}

	fmt.Print("generating expected outputs...")

	for _, script := range scripts {
		if script.IsDir() {
			continue
		}
		name := script.Name()

		cmd := exec.Command(EXPECTED_TEST_RESULT_RUNNER, DIR_NAME_TEST_SCRIPTS+"/"+name)
		out, err := cmd.CombinedOutput()

		if err != nil {
			log.Fatalf("failed to run test script %s %s", name, err)
		}

		expected[name] = out
	}

	fmt.Print(" done")

	fmt.Printf("\nRunning %d tests.\n", len(expected))

	fails := []struct {
		test     string
		expected string
		got      string
	}{}

	for nth, script := range scripts {
		if script.IsDir() {
			continue
		}
		name := script.Name()
		fmt.Printf("%-50s %2d/%d... ", name, nth+1, len(expected))

		cmd := exec.Command("./"+RUNTIME_BINARY_NAME, "test-scripts/"+name)
		out, err := cmd.CombinedOutput()

		if err != nil {
			fmt.Println("❌")
			fails = append(fails, struct {
				test     string
				expected string
				got      string
			}{test: name, expected: string(expected[name]), got: err.Error()})
			continue
		}

		if bytes.Equal(expected[name], out) {
			fmt.Println("✅")
		} else {
			fmt.Println("❌")
			fails = append(fails, struct {
				test     string
				expected string
				got      string
			}{test: name, expected: string(expected[name]), got: string(out)})
		}
	}

	if len(fails) > 0 {
		fmt.Println()
		fmt.Println("FAILED TESTS")
		fmt.Println()
		for _, result := range fails {
			fmt.Printf("Test case: '%s'\n\n", result.test)
			fmt.Printf("-- Expected --\n%s", result.expected)
			fmt.Printf("-- Got --\n%s", result.got)
			fmt.Println()
			fmt.Printf("%s\n", strings.Repeat("-", 34))
		}
	}

	fmt.Printf("\n#####\n\n%d/%d Tests passed\n\n#####\n\n\n", len(scripts)-len(fails), len(scripts))

	fmt.Print("RUNNING BENCHMARKS")

	results := []struct {
		test     string
		expected string
		got      string
	}{}

	for _, bench := range benchmarks {
		name := bench.Name()
		path := DIR_NAME_BENCMARKS + "/" + name

		nodeCmd := exec.Command(EXPECTED_TEST_RESULT_RUNNER, path)
		standardOut, err := nodeCmd.CombinedOutput()

		if err != nil {
			log.Fatalf("failed to run benchmark script %s %s", name, err)
		}

		goJsCmd := exec.Command("./"+RUNTIME_BINARY_NAME, path)

		attemptOut, err := goJsCmd.CombinedOutput()

		if err != nil {
			log.Fatalf("failed to run benchmark script %s %s", name, err)
		}

		results = append(results, struct {
			test     string
			expected string
			got      string
		}{test: name, expected: string(standardOut), got: string(attemptOut)})
		fmt.Print(".")
	}
	fmt.Println("Done!")
	fmt.Println()
	fmt.Println()

	for _, result := range results {
		fmt.Printf("Benchmark: '%s'\n\n", result.test)
		fmt.Printf("-- NodeJS performance --\n%s", result.expected)
		fmt.Printf("-- Got --\n%s", result.got)
		fmt.Println()
		fmt.Printf("%s\n", strings.Repeat("-", 34))
	}
}
