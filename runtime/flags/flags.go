package flags

import "runtime"

var (
	Debug            = false
	EnableJIT        = runtime.GOARCH == "amd64"
	StructuredOutput = true
)
