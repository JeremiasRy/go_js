package flags

import (
	"fmt"
	"runtime"
)

const (
	JITOff   = "off"
	JITAuto  = "auto"
	JITForce = "force"
)

var (
	Debug            = false
	EnableJIT        = runtime.GOARCH == "amd64"
	JITMode          = defaultJITMode()
	JITCallThreshold = 4096
	JITLoopThreshold = 4096
	JITMaxCodeBytes  = 16 << 20
	JITStats         = false
	JITTrace         = false
	StructuredOutput = true
	JSONErrors       = false
	TraceExecution   = false
	TraceMaxFrames   = 10000
)

func defaultJITMode() string {
	if runtime.GOARCH == "amd64" {
		return JITAuto
	}
	return JITOff
}

func SetJITMode(mode string) error {
	switch mode {
	case JITOff, JITAuto, JITForce:
	default:
		return fmt.Errorf("invalid jit mode %q: expected off, auto or force", mode)
	}
	JITMode = mode
	EnableJIT = mode != JITOff && runtime.GOARCH == "amd64"
	return nil
}
