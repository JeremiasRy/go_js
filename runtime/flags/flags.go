package flags

import "runtime"

var Debug = false
var ENABLE_JIT = runtime.GOOS == "linux"
