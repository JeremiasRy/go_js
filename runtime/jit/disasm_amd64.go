//go:build amd64

package jit

import (
	"bytes"
	"fmt"
	"go_js/flags"
	"unsafe"

	"github.com/zyantific/zydis-go"
)

//go:linkname zydisDisassembleIntel github.com/zyantific/zydis-go.__imp_ZydisDisassembleIntel
var zydisDisassembleIntel uintptr

func disassembleIntel(runtimeAddress uint64, buffer unsafe.Pointer, length uint64, insn *zydis.DisassembledInstruction) zydis.Status {
	return zydis.Status(call5Aligned(
		zydisDisassembleIntel,
		uintptr(zydis.MACHINE_MODE_LONG_64),
		uintptr(runtimeAddress),
		uintptr(buffer),
		uintptr(length),
		uintptr(unsafe.Pointer(insn)),
	))
}

func buildDisassembly(code []byte, mem []byte, markers []codeMarker) []JitDetail {
	if !flags.StructuredOutput || len(code) == 0 {
		return nil
	}
	details := make([]JitDetail, 0, 128)
	insn := zydis.DisassembledInstruction{}
	base := uintptr(unsafe.Pointer(&mem[0]))
	offset := 0
	for offset < len(code) {
		status := disassembleIntel(
			uint64(base+uintptr(offset)),
			unsafe.Pointer(&code[offset]),
			uint64(len(code)-offset),
			&insn,
		)
		if !zydis.Ok(status) || insn.Info.Length == 0 {
			break
		}
		marker := codeMarker{opPtr: -1, astID: -1}
		lo, hi := 0, len(markers)-1
		for lo <= hi {
			mid := (lo + hi) / 2
			if markers[mid].offset <= offset {
				marker = markers[mid]
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		textEnd := bytes.IndexByte(insn.Text[:], 0)
		if textEnd < 0 {
			textEnd = len(insn.Text)
		}
		details = append(details, JitDetail{
			Inst:      fmt.Sprintf("%016X  %s\n", base+uintptr(offset), string(insn.Text[:textEnd])),
			OpCodePtr: marker.opPtr,
			AstId:     marker.astID,
		})
		offset += int(insn.Info.Length)
	}
	return details
}
