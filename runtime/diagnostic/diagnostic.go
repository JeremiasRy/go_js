package diagnostic

import (
	"encoding/json"
	"go_js/flags"
	"log"
	"os"
)

// Fatal reports an engine diagnostic without mixing it into program stdout.
// A parser rejection is a SyntaxError; internal compiler/VM errors retain the
// generic Error class so they cannot satisfy a typed negative test by accident.
func Fatal(phase, name string, err error) {
	if flags.JSONErrors {
		json.NewEncoder(os.Stderr).Encode(struct {
			Outcome string `json:"outcome"`
			Phase   string `json:"phase"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}{"error", phase, name, err.Error()})
	} else {
		log.Printf("%s error: %s: %s", phase, name, err)
	}
	os.Exit(1)
}
