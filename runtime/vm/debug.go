package vm

import (
	"fmt"
	"go_js/jit"
	"sort"
	"strings"

	"go_js/chunk"
	"go_js/compiler"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/parser"
	"go_js/value"
)

const StructuredOutputFormatVersion = 2
const ExecutionSchemaVersion = 1

type StructuredOut struct {
	FormatVersion int                         `json:"format_version"`
	Output        string                      `json:"output"`
	DebugInfo     map[string]*LegacyDebugInfo `json:"debug_info"`
	DebugChunks   []*ChunkInfo                `json:"debug_chunks"`
	Ast           *parser.Node                `json:"ast"`
	Execution     *ExecutionInfo              `json:"execution"`
}

type LegacyDebugInfo struct {
	Jit      []jit.JitDetail `json:"jit"`
	ByteCode []OpDetail      `json:"byte_code"`
}

type ExecutionLimits struct {
	MaxFrames     int `json:"max_frames"`
	MaxStack      int `json:"max_stack"`
	MaxQueueItems int `json:"max_queue_items"`
	MaxTimers     int `json:"max_timers"`
	MaxValueChars int `json:"max_value_chars"`
}

type ExecutionInfo struct {
	SchemaVersion int             `json:"schema_version"`
	Status        string          `json:"status"`
	Frames        []any           `json:"frames"`
	Truncated     bool            `json:"truncated"`
	TotalFrames   int             `json:"total_frames"`
	DroppedFrames int             `json:"dropped_frames"`
	DurationMS    float64         `json:"duration_ms"`
	Limits        ExecutionLimits `json:"limits"`
}

func NewExecutionPlaceholder() *ExecutionInfo {
	return &ExecutionInfo{
		SchemaVersion: ExecutionSchemaVersion,
		Status:        "not_requested",
		Frames:        []any{},
		Limits: ExecutionLimits{
			MaxFrames:     10000,
			MaxStack:      64,
			MaxQueueItems: 32,
			MaxTimers:     16,
			MaxValueChars: 160,
		},
	}
}

const NO_MORE_OPS = -1

func opName(code uint8) string {
	if name, found := chunk.OpNames[code]; found {
		return name
	}
	return fmt.Sprintf("OP_UNKNOWN_%d", code)
}

func readUint32At(code []uint8, ip int) (int, bool) {
	if ip+4 > len(code) {
		return 0, false
	}
	return int(code[ip])<<24 | int(code[ip+1])<<16 | int(code[ip+2])<<8 | int(code[ip+3]), true
}

func ReadOp(ip int, c value.ValueChunk, sb *strings.Builder) (int, bool) {
	opCode := c.Code
	if ip >= len(opCode) {
		return ip, false
	}
	code := opCode[ip]
	next := ip + 1

	fmt.Fprintf(sb, "%04d | %s \n", ip*4, opName(code))

	switch code {
	case chunk.OP_JUMP_IF_FALSE, chunk.OP_JUMP_IF_TRUE, chunk.OP_JUMP,
		chunk.OP_JUMP_IF_NULLISH, chunk.OP_JUMP_IF_NOT_NULLISH,
		chunk.OP_JUMP_IF_NOT_UNDEFINED, chunk.OP_TRY_BLOCK_START,
		chunk.OP_TRY_FINALLY_START:
		operand, ok := readUint32At(opCode, next)
		if !ok {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | %d\n", next*4, operand*4)
		next += 4

	case chunk.OP_CREATE_ARRAY:
		length, ok := readUint32At(opCode, next)
		if !ok {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | %d\n", next*4, length)
		next += 4

	case chunk.OP_CALL, chunk.OP_CALL_OPTIONAL, chunk.OP_CALL_CALLEE_FIRST:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | %d \n", next*4, opCode[next])
		next++
		spread := "false"
		if opCode[next] != 0 {
			spread = "true"
		}
		fmt.Fprintf(sb, "%04d | spread: %s \n", next*4, spread)
		next++

	case chunk.OP_GET_HEAP_VAR, chunk.OP_SET_HEAP_VAR, chunk.OP_GET_HEAP_VAR_RAW:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | depth: %d \n", next*4, opCode[next])
		next++
		fmt.Fprintf(sb, "%04d | slot: %d \n", next*4, opCode[next])
		next++

	case chunk.OP_SET_GLOBAL_NAMED:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | slot: %d \n", next*4, opCode[next])
		next++
		fmt.Fprintf(sb, "%04d | name: %d \n", next*4, opCode[next])
		next++

	case chunk.OP_MEMBER_UPDATE:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		prefix := "false"
		if opCode[next] == 1 {
			prefix = "true"
		}
		fmt.Fprintf(sb, "%04d | prefix: %s \n", next*4, prefix)
		next++
		delta := "+1"
		if opCode[next] == 1 {
			delta = "-1"
		}
		fmt.Fprintf(sb, "%04d | delta: %s \n", next*4, delta)
		next++

	case chunk.OP_NEW_REGEXP:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | pattern: %d \n", next*4, opCode[next])
		next++
		fmt.Fprintf(sb, "%04d | flags: %d \n", next*4, opCode[next])
		next++

	case chunk.OP_PUSH_METHOD:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | flags: %d \n", next*4, opCode[next])
		next++
		fmt.Fprintf(sb, "%04d | kind: %d \n", next*4, opCode[next])
		next++

	case chunk.OP_SUPER_CALL:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | arguments: %d \n", next*4, opCode[next])
		next++
		fmt.Fprintf(sb, "%04d | %d \n", next*4, opCode[next])
		next++

	case chunk.OP_GET_ITERATOR:
		if next >= len(opCode) {
			return len(opCode), true
		}
		t := opCode[next]
		str := "ITERATOR_FOR_OF"
		if t == compiler.ITERATOR_FOR_IN {
			str = "ITERATOR_FOR_IN"
		}
		fmt.Fprintf(sb, "%04d | %d -> %s \n", next*4, t, str)
		next++

	case chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS:
		if next >= len(opCode) {
			return len(opCode), true
		}
		amount := opCode[next]
		fmt.Fprintf(sb, "%04d | amount: %d \n", next*4, amount)
		next++
		for range amount {
			if next+2 > len(opCode) {
				return len(opCode), true
			}
			fmt.Fprintf(sb, "%04d | var index: %d \n", next*4, opCode[next])
			next++
			fmt.Fprintf(sb, "%04d | var slot: %d \n", next*4, opCode[next])
			next++
		}

	case chunk.OP_CREATE_REST_OBJECT:
		if next >= len(opCode) {
			return len(opCode), true
		}
		amount := opCode[next]
		fmt.Fprintf(sb, "%04d | %d \n", next*4, amount)
		next++
		for range amount {
			if next >= len(opCode) {
				return len(opCode), true
			}
			idx := opCode[next]
			label := fmt.Sprintf("%d", idx)
			if int(idx) < len(c.Constants) {
				label = native.String(c.Constants[idx])
			}
			fmt.Fprintf(sb, "%04d | %s \n", next*4, label)
			next++
		}

	case chunk.OP_CONSTANT, chunk.OP_GET_GLOBAL, chunk.OP_SET_GLOBAL,
		chunk.OP_GET_LOCAL, chunk.OP_SET_LOCAL, chunk.OP_DEFINE_LOCAL_SLOT,
		chunk.OP_GET_LOCAL_RAW, chunk.OP_GET_GLOBAL_BY_NAME,
		chunk.OP_DEFINE_GLOBAL_NAMED, chunk.OP_NEW,
		chunk.OP_DEFINE_HEAP_VAR_AT, chunk.OP_DEFINE_ARGUMENTS_HEAP,
		chunk.OP_CATCH_BIND, chunk.OP_GET_CATCH, chunk.OP_SET_CATCH,
		chunk.OP_ITERATOR_NEXT_SLOT, chunk.OP_ITERATOR_CURRENT_SLOT,
		chunk.OP_ITERATOR_CLOSE_SLOT, chunk.OP_ITERATOR_CLOSE_MARK,
		chunk.OP_ASYNC_ITERATOR_NEXT_SLOT, chunk.OP_ASYNC_ITERATOR_RESULT_SLOT,
		chunk.OP_ASYNC_ITERATOR_VALUE_SLOT, chunk.OP_ASYNC_ITERATOR_CLOSE_SLOT,
		chunk.OP_TRIM_STACK_SLOT, chunk.OP_STACK_MARK, chunk.OP_STACK_TRIM,
		chunk.OP_STACK_UNMARK, chunk.OP_CREATE_REST_OBJECT_DYNAMIC,
		chunk.OP_PUSH_CLASS_FIELD, chunk.OP_UPDATE:
		if next >= len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | %d \n", next*4, opCode[next])
		next++
	}

	return next, true
}

func PrintFunction(c value.ValueChunk, sb *strings.Builder) {
	ip, more := ReadOp(0, c, sb)
	for more {
		ip, more = ReadOp(ip, c, sb)
	}

	fmt.Fprintln(sb)

	for _, value := range c.Constants {
		if value.IsObject() {
			obj, _ := heap.GetObject(value.GetHandle())
			switch f := obj.(type) {
			case object.Callable:
				PrintFunction(*f.ValueChunk(), sb)
			}
		}
	}
}

func PrintChunk(c value.ValueChunk) {
	sb := &strings.Builder{}
	PrintFunction(c, sb)
	println(sb.String())
}

type OpDetail struct {
	Op    string `json:"op"`
	AstId int    `json:"ast_id"`
	OpPtr int    `json:"op_ptr"`
}

type ChunkInfo struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Kind     string          `json:"kind,omitempty"`
	Hidden   bool            `json:"hidden,omitempty"`
	ByteCode []OpDetail      `json:"byte_code"`
	Jit      []jit.JitDetail `json:"jit"`
}

type DebugInfo struct {
	Chunks []*ChunkInfo
	Legacy map[string]*LegacyDebugInfo

	seen map[*value.ValueChunk]*ChunkInfo
	byID map[string]*ChunkInfo
}

func NewDebugInfo() *DebugInfo {
	return &DebugInfo{
		Chunks: []*ChunkInfo{},
		Legacy: map[string]*LegacyDebugInfo{},
		seen:   map[*value.ValueChunk]*ChunkInfo{},
		byID:   map[string]*ChunkInfo{},
	}
}

func (d *DebugInfo) AddCallable(fn object.Callable, nameOverride string, hideSameName bool) *ChunkInfo {
	if fn == nil {
		return nil
	}
	if d.seen == nil {
		d.seen = map[*value.ValueChunk]*ChunkInfo{}
	}
	if d.byID == nil {
		d.byID = map[string]*ChunkInfo{}
	}
	if d.Legacy == nil {
		d.Legacy = map[string]*LegacyDebugInfo{}
	}
	return d.addChunkTree(fn.ValueChunk(), callableKind(fn, nameOverride), nameOverride, hideSameName)
}

func (d *DebugInfo) addChunkTree(c *value.ValueChunk, kind, nameOverride string, hideSameName bool) *ChunkInfo {
	if c == nil {
		return nil
	}
	if info, found := d.seen[c]; found {
		d.applyName(info, nameOverride, hideSameName)
		return info
	}

	info := d.addChunk(c, kind, nameOverride, hideSameName)
	d.seen[c] = info

	for _, constant := range c.Constants {
		if !constant.IsObject() {
			continue
		}
		obj, err := heap.GetObject(constant.GetHandle())
		if err != nil {
			continue
		}
		fn, ok := obj.(object.Callable)
		if !ok {
			continue
		}
		d.addChunkTree(fn.ValueChunk(), callableKind(fn, ""), "", false)
	}

	return info
}

func (d *DebugInfo) addChunk(c *value.ValueChunk, kind, nameOverride string, hideSameName bool) *ChunkInfo {
	if c.ChunkID == "" {
		c.ChunkID = value.NextChunkID()
	}
	if info, found := d.byID[c.ChunkID]; found {
		d.applyName(info, nameOverride, hideSameName)
		return info
	}

	name := c.FnName
	if nameOverride != "" {
		name = nameOverride
	}
	if kind == "" {
		kind = chunkNameKind(name)
	}

	info := &ChunkInfo{
		ID:       c.ChunkID,
		Name:     name,
		Kind:     kind,
		Hidden:   kind == "setup" || name == object.SETUP_FN_NAME,
		ByteCode: []OpDetail{},
		Jit:      []jit.JitDetail{},
	}

	ip := 0
	sb := &strings.Builder{}
	for ip < len(c.Code) {
		opcodeIp := ip
		astId := c.AstId[ip]
		ip, _ = ReadOp(ip, *c, sb)
		info.ByteCode = append(info.ByteCode, OpDetail{Op: sb.String(), AstId: astId, OpPtr: opcodeIp})
		sb.Reset()
	}

	if found, disassembly := jitContext().Disassembly(c); found {
		info.Jit = disassembly
	}

	d.Chunks = append(d.Chunks, info)
	d.byID[c.ChunkID] = info

	if hideSameName {
		d.hideOtherChunksNamed(name, c.ChunkID)
	}

	return info
}

func (d *DebugInfo) applyName(info *ChunkInfo, nameOverride string, hideSameName bool) {
	if info == nil || nameOverride == "" {
		return
	}
	info.Name = nameOverride
	if nameOverride == object.MAIN_FN_NAME && info.Kind != "setup" {
		info.Kind = "main"
	}
	if hideSameName {
		d.hideOtherChunksNamed(nameOverride, info.ID)
	}
}

func (d *DebugInfo) hideOtherChunksNamed(name, keepID string) {
	for _, chunk := range d.Chunks {
		if chunk.ID != keepID && chunk.Name == name {
			chunk.Hidden = true
		}
	}
}

func (d *DebugInfo) Finalize() {
	if d.Legacy == nil {
		d.Legacy = map[string]*LegacyDebugInfo{}
	}
	for _, chunk := range d.Chunks {
		d.Legacy[chunk.Name] = &LegacyDebugInfo{
			Jit:      chunk.Jit,
			ByteCode: chunk.ByteCode,
		}
	}

	sort.Slice(d.Chunks, func(i, j int) bool {
		if d.Chunks[i].Name == d.Chunks[j].Name {
			return d.Chunks[i].ID < d.Chunks[j].ID
		}
		return d.Chunks[i].Name < d.Chunks[j].Name
	})
}

func callableKind(fn object.Callable, nameOverride string) string {
	name := nameOverride
	if name == "" {
		name = fn.ValueChunk().FnName
	}
	if name == object.SETUP_FN_NAME {
		return "setup"
	}
	if name == object.MAIN_FN_NAME {
		return "main"
	}
	if strings.HasPrefix(name, "MODULE ") {
		return "module"
	}

	switch fn := fn.(type) {
	case *native.ObjAsyncFunction:
		return "async_function"
	case *native.ObjGenerator:
		if fn.IsAsync {
			return "async_generator"
		}
		return "generator"
	default:
		return "function"
	}
}

func chunkNameKind(name string) string {
	if name == object.SETUP_FN_NAME {
		return "setup"
	}
	if name == object.MAIN_FN_NAME {
		return "main"
	}
	if strings.HasPrefix(name, "MODULE ") {
		return "module"
	}
	return "function"
}

func printStack(stack []value.Value) {
	print("[")
	for i, val := range stack {
		fmt.Printf("%s", strings.ReplaceAll(native.TypeDecoratedString(val, nil), "\n", "\\n"))
		if i < len(stack)-1 {
			fmt.Print(" | ")
		}
	}
	println(" ]")
}
