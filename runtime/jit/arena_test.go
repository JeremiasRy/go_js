package jit

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"go_js/chunk"
	"go_js/object"
	"go_js/value"
)

func simpleChunk(name string) *value.ValueChunk {
	ch := value.NewChunk(name)
	ch.Constants = []value.Value{value.ValueFromFloat64(1)}
	ch.Code = []uint8{
		chunk.OP_CONSTANT, 0,
		chunk.OP_RETURN,
	}
	ch.AstId = make([]int, len(ch.Code))
	return ch
}

func TestCodeArenaLimitFailsCleanly(t *testing.T) {
	ctx := NewContext(4096)
	if _, err := ctx.compileLocked(simpleChunk("a"), 0); err != nil {
		t.Fatalf("first compile failed: %v", err)
	}
	if _, err := ctx.compileLocked(simpleChunk("b"), 0); err == nil {
		t.Fatal("second compile exceeded the code limit without an error")
	}
}

func TestCodePagesAreNotWritable(t *testing.T) {
	if _, err := os.Stat("/proc/self/maps"); err != nil {
		t.Skip("no /proc/self/maps on this platform")
	}
	ctx := NewContext(1 << 20)
	mem, err := ctx.seal([]byte{0xC3})
	if err != nil {
		t.Fatal(err)
	}
	addr := uintptr(unsafe.Pointer(&mem[0]))
	data, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		var start, end uintptr
		if _, err := fmt.Sscanf(fields[0], "%x-%x", &start, &end); err != nil {
			continue
		}
		if addr >= start && addr < end {
			found = true
			if strings.Contains(fields[1], "w") || !strings.Contains(fields[1], "x") {
				t.Fatalf("JIT code page has permissions %q, want executable and not writable", fields[1])
			}
		}
	}
	if !found {
		t.Fatalf("could not find the JIT code page for %x", addr)
	}
}

func TestConcurrentCompile(t *testing.T) {
	ctx := NewContext(1 << 20)
	ch := simpleChunk("shared")
	fn := object.NewFunction("shared", 0, ch)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := ctx.Compile(fn); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent compile: %v", err)
	}
}

func TestManyChunks(t *testing.T) {
	ctx := NewContext(64 << 20)
	for i := 0; i < 1000; i++ {
		ch := simpleChunk(fmt.Sprintf("f%d", i))
		if _, err := ctx.compileLocked(ch, 0); err != nil {
			t.Fatalf("chunk %d failed: %v", i, err)
		}
	}
	if ctx.codeBytes < 1000*4096 {
		t.Fatalf("code arena did not grow as expected: %d bytes", ctx.codeBytes)
	}
}

func FuzzDecodeChunk(f *testing.F) {
	f.Add([]byte{chunk.OP_CONSTANT, 0, chunk.OP_RETURN})
	f.Add([]byte{chunk.OP_JUMP, 0, 0, 0, 4})
	f.Add([]byte{chunk.OP_CALL, 1, 0})
	f.Add([]byte{chunk.OP_CREATE_REST_OBJECT, 2, 1})
	f.Fuzz(func(t *testing.T, code []byte) {
		_, _ = decodeChunk(code)
	})
}
