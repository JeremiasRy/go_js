package main

import (
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
		if err.Error() == "mkdir expected: file exists" {
			goto Continue
		}
		log.Fatalf("Failed to create %s folder %s", DIR_NAME_EXPECTED, err)
	}

Continue:

	scripts, err := os.ReadDir(DIR_NAME_TEST_SCRIPTS)

	if err != nil {
		log.Fatalf("failed to read test scripts %s", err)
	}

	expected := map[string][]byte{}

	println("generating expected outputs...")

	for _, script := range scripts {
		name := script.Name()

		if err != nil {
			log.Fatalf("failed to read script order number %s", err)
		}

		cmd := exec.Command("node", "test-scripts/"+name)
		out, err := cmd.CombinedOutput()

		if err != nil {
			log.Fatalf("failed to run test script %s", err)
		}

		expected[name] = out
	}

	println("generated expected outputs")
	fmt.Printf("Running %d tests...\n", len(expected))

	for _, script := range scripts {
		name := script.Name()

		if err != nil {
			log.Fatalf("failed to read script order number %s", err)
		}

		cmd := exec.Command("./"+RUNTIME_BINARY_NAME, "test-scripts/"+name)
		out, err := cmd.CombinedOutput()

		if err != nil {
			log.Fatalf("failed to run test script %s", err)
		}

		if string(expected[name]) != string(out) {
			println("failed test", name)
			fmt.Printf("Expected:\n%sGot:\n%s", expected[name], out)
		}
	}

	err = os.RemoveAll(DIR_NAME_EXPECTED)

	if err != nil {
		log.Fatalf("Failed to delete %s folder %s", DIR_NAME_EXPECTED, err)
	}
}
