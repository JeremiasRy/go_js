package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

const EXPECTED_DIR_NAME = "expected"

func main() {
	err := os.Mkdir(EXPECTED_DIR_NAME, 0755)

	if err != nil {
		if err.Error() == "mkdir expected: file exists" {
			goto Continue
		}
		log.Fatalf("Failed to create %s folder %s", EXPECTED_DIR_NAME, err)
	}

Continue:
	cmd := exec.Command("node", "testScripts/mission1.js")
	out, err := cmd.CombinedOutput()

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Node output:\n%s\n", out)
	err = os.RemoveAll(EXPECTED_DIR_NAME)

	if err != nil {
		log.Fatalf("Failed to delete %s folder %s", EXPECTED_DIR_NAME, err)
	}
}
