package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
)

const DIR_NAME_EXPECTED = "expected"
const DIR_NAME_TEST_SCRIPTS = "test-scripts"

const PREFIX_TEST_SCIPT = "mission"

const EXPECTED_TEST_RESULT_RUNNER = "node"
const RUNTIME_BINARY_NAME = "go_js"

func main() {
	err := os.Mkdir(DIR_NAME_EXPECTED, 0755)

	if err != nil {
		log.Fatalf("Failed to create %s folder %s", DIR_NAME_EXPECTED, err)
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

		if err != nil {
			log.Fatalf("failed to read script order number %s", err)
		}

		cmd := exec.Command("node", "test-scripts/"+name)
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
		fmt.Printf("%-12s %2d/%d...", name, nth+1, len(expected))

		if err != nil {
			log.Fatalf("failed to read script order number %s", err)
		}

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
		indent := "		"
		for _, result := range fails {
			fmt.Printf("Test case: '%s'\n\n", result.test)
			fmt.Printf("%s-- Expected --\n%s\n", indent, result.expected)
			fmt.Printf("%s-- Got --\n%s\n", indent, result.got)
			fmt.Println("--------")
		}
	}

	err = os.RemoveAll(DIR_NAME_EXPECTED)

	if err != nil {
		log.Fatalf("Failed to delete %s folder %s", DIR_NAME_EXPECTED, err)
	}
}
