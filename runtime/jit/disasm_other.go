//go:build !amd64

package jit

func buildDisassembly(code []byte, mem []byte, markers []codeMarker) []JitDetail {
	return nil
}
