package flags

import "runtime"

var (
	Debug            = false
	EnableJIT        = runtime.GOOS == "linux"
	StructuredOutput = true
)
