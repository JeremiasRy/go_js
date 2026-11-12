package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const referenceResultMarker = "\n@@GO_JS_TEST_RESULT@@"
const maxOutputBytes = 1024 * 1024

type limitedBuffer struct {
	buffer    bytes.Buffer
	truncated bool
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	remaining := maxOutputBytes - buffer.buffer.Len()
	if len(data) > remaining {
		buffer.buffer.Write(data[:remaining])
		buffer.truncated = true
	} else {
		buffer.buffer.Write(data)
	}
	return len(data), nil
}

func (buffer *limitedBuffer) Bytes() []byte {
	return buffer.buffer.Bytes()
}

type diagnostic struct {
	Outcome string `json:"outcome"`
	Phase   string `json:"phase"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type execution struct {
	Output     []byte
	Stderr     []byte
	Diagnostic *diagnostic
	Err        error
}

func execute(timeout time.Duration, name string, args ...string) execution {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	var stdout, stderr limitedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		err = fmt.Errorf("process timed out after %s", timeout)
	} else if stdout.truncated || stderr.truncated {
		err = fmt.Errorf("process output exceeded %d bytes", maxOutputBytes)
	}
	return execution{Output: stdout.Bytes(), Stderr: stderr.Bytes(), Err: err}
}

func runNode(test testCase, timeout time.Duration) execution {
	result := execute(timeout, EXPECTED_TEST_RESULT_RUNNER,
		"--experimental-vm-modules", "--no-warnings", "--unhandled-rejections=strict",
		"node-reference.js", test.SourceGoal, test.File)
	return decodeReference(result)
}

func decodeReference(result execution) execution {
	if result.Err != nil {
		return result
	}
	if len(result.Stderr) != 0 {
		result.Err = fmt.Errorf("reference process wrote unexpected stderr")
		return result
	}
	marker := bytes.LastIndex(result.Output, []byte(referenceResultMarker))
	if marker < 0 {
		result.Err = fmt.Errorf("reference process exited without completing the case")
		return result
	}
	var report diagnostic
	err := json.Unmarshal(result.Output[marker+len(referenceResultMarker):], &report)
	result.Output = result.Output[:marker]
	if err != nil {
		result.Err = fmt.Errorf("invalid reference result: %w", err)
		return result
	}
	switch report.Outcome {
	case "success":
	case "error":
		if report.Phase == "" || report.Error == "" {
			result.Err = fmt.Errorf("reference error is missing its phase or class")
		} else {
			result.Diagnostic = &report
		}
	default:
		result.Err = fmt.Errorf("unknown reference outcome %q", report.Outcome)
	}
	return result
}

func runRuntime(test testCase, timeout time.Duration) execution {
	result := execute(timeout, "./"+RUNTIME_BINARY_NAME,
		"--json-errors", "--source-type="+test.SourceGoal, test.File)
	return decodeRuntime(result)
}

func decodeRuntime(result execution) execution {
	if result.Err == nil {
		if len(result.Stderr) != 0 {
			result.Err = fmt.Errorf("runtime wrote unexpected stderr")
		}
		return result
	}
	// Only an explicit engine diagnostic on a normal error exit is a JS error.
	// Panics, signals, timeouts, and launch failures remain test failures.
	var exitError *exec.ExitError
	if !errors.As(result.Err, &exitError) || exitError.ExitCode() != 1 {
		return result
	}
	var report diagnostic
	if json.Unmarshal(result.Stderr, &report) == nil && report.Outcome == "error" && report.Phase != "" && report.Error != "" {
		result.Diagnostic = &report
		result.Err = nil
		result.Stderr = nil
	}
	return result
}

func checkOutcome(test testCase, result execution) error {
	if result.Err != nil {
		return result.Err
	}
	if test.Error != "" {
		if result.Diagnostic == nil {
			return fmt.Errorf("expected %s %s, but execution succeeded", test.Phase, test.Error)
		}
		if result.Diagnostic.Phase != test.Phase || result.Diagnostic.Error != test.Error {
			return fmt.Errorf("expected %s %s, got %s %s", test.Phase, test.Error, result.Diagnostic.Phase, result.Diagnostic.Error)
		}
	} else if result.Diagnostic != nil {
		return fmt.Errorf("unexpected %s %s: %s", result.Diagnostic.Phase, result.Diagnostic.Error, result.Diagnostic.Message)
	}
	return nil
}

func compareCase(test testCase, expected, got execution) error {
	if err := checkOutcome(test, expected); err != nil {
		return fmt.Errorf("Node reference: %w", err)
	}
	if err := checkOutcome(test, got); err != nil {
		return fmt.Errorf("GO_JS: %w", err)
	}
	if !bytes.Equal(expected.Output, got.Output) {
		return fmt.Errorf("stdout differs from Node")
	}
	return nil
}

func describeExecution(result execution) string {
	var description strings.Builder
	if result.Err != nil {
		fmt.Fprintln(&description, result.Err)
	}
	if result.Diagnostic != nil {
		fmt.Fprintf(&description, "%s %s: %s\n", result.Diagnostic.Phase, result.Diagnostic.Error, result.Diagnostic.Message)
	}
	for _, output := range [][]byte{result.Output, result.Stderr} {
		if len(output) > 8192 {
			description.Write(output[:8192])
			description.WriteString("\n[output truncated]\n")
		} else {
			description.Write(output)
		}
	}
	return description.String()
}
