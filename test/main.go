package main

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const DIR_NAME_EXPECTED = "expected"
const DIR_NAME_TEST_SCRIPTS = "test-scripts"

const PREFIX_TEST_SCIPT = "mission"

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

	for _, script := range scripts {
		name := script.Name()

		split := strings.Split(strings.Split(name, ".js")[0], "mission")
		nth, err := strconv.Atoi(split[len(split)-1])

		if err != nil {
			log.Fatalf("failed to read script order number %s", err)
		}

		cmd := exec.Command("node", "test-scripts/"+name)
		expected, err := cmd.CombinedOutput()

		if err != nil {
			log.Fatalf("failed to run test script %s", err)
		}

		file, err := os.Create(DIR_NAME_EXPECTED + "/expected_" + strconv.Itoa(nth))

		if err != nil {
			log.Fatalf("failed to write expected result to a file %s", err)
		}
		file.Write(expected)
	}

	err = os.RemoveAll(DIR_NAME_EXPECTED)

	if err != nil {
		log.Fatalf("Failed to delete %s folder %s", DIR_NAME_EXPECTED, err)
	}
}
