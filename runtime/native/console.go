package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

type Console struct {
	ObjObject
	timers map[string]time.Time
	counts map[string]int
}

func (*Console) Type() object.ObjType {
	return object.OBJ_CONSOLE
}

func (*Console) String() string {
	return "Object [console]"
}

func NewObjectConsole() *Console {
	c := &Console{timers: map[string]time.Time{}, counts: map[string]int{}}
	c.Members = map[string]ObjectValueEntry{}
	c.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	log := value.EncodeHandle(heap.Allocate(NewLightString("log")))

	c.SetMember(log, value.EncodeHandle(heap.Allocate(NewLog())))
	for _, item := range []struct {
		name string
		kind ConsoleMethodKind
	}{
		{"error", CONSOLE_ERROR},
		{"warn", CONSOLE_WARN},
		{"info", CONSOLE_INFO},
		{"debug", CONSOLE_DEBUG},
		{"assert", CONSOLE_ASSERT},
		{"dir", CONSOLE_DIR},
		{"trace", CONSOLE_TRACE},
		{"count", CONSOLE_COUNT},
		{"group", CONSOLE_GROUP},
		{"groupEnd", CONSOLE_GROUP_END},
		{"table", CONSOLE_TABLE},
		{"time", CONSOLE_TIME},
		{"timeEnd", CONSOLE_TIME_END},
	} {
		c.SetMember(value.EncodeHandle(heap.Allocate(NewLightString(item.name))), value.EncodeHandle(heap.Allocate(NewConsoleMethod(item.name, item.kind))))
	}
	return c
}

type ConsoleMethodKind int

const (
	CONSOLE_ERROR ConsoleMethodKind = iota
	CONSOLE_TABLE
	CONSOLE_TIME
	CONSOLE_TIME_END
	CONSOLE_WARN
	CONSOLE_INFO
	CONSOLE_DEBUG
	CONSOLE_ASSERT
	CONSOLE_DIR
	CONSOLE_TRACE
	CONSOLE_COUNT
	CONSOLE_GROUP
	CONSOLE_GROUP_END
)

type ConsoleMethod struct {
	ObjNativeFn
	Kind ConsoleMethodKind
}

func NewConsoleMethod(name string, kind ConsoleMethodKind) *ConsoleMethod {
	m := &ConsoleMethod{Kind: kind}
	m.name = name
	return m
}

func (c *Console) Start(label string) {
	if c.timers == nil {
		c.timers = map[string]time.Time{}
	}
	c.timers[label] = time.Now()
}

func (c *Console) End(label string) (time.Duration, bool) {
	start, found := c.timers[label]
	return time.Since(start), found
}

func (c *Console) Count(label string) int {
	if c.counts == nil {
		c.counts = map[string]int{}
	}
	c.counts[label]++
	return c.counts[label]
}

func ConsoleTable(values []value.Value, output *strings.Builder) {
	if len(values) == 0 {
		return
	}
	if len(values) == 1 && values[0].IsObject() {
		if obj, err := heap.GetObject(values[0].GetHandle()); err == nil {
			if arr, ok := obj.(*ObjArr); ok {
				items := make([]value.Value, arr.Length())
				for i := 0; i < arr.Length(); i++ {
					items[i] = arr.GetElementAt(i)
				}
				values = items
			}
		}
	}
	rows := []*ObjObject{}
	for _, item := range values {
		if !item.IsObject() {
			continue
		}
		obj, err := heap.GetObject(item.GetHandle())
		if err != nil {
			continue
		}
		if o, ok := obj.(*ObjObject); ok {
			rows = append(rows, o)
		}
	}
	if len(rows) == 0 {
		return
	}
	columns := ObjectEnumerableKeys(rows[0])
	widths := make([]int, len(columns))
	for i, column := range columns {
		widths[i] = len(column)
	}
	indexWidth := len("(index)")
	cellValues := make([][]string, len(rows))
	for r, row := range rows {
		cellValues[r] = make([]string, len(columns))
		index := strconv.Itoa(r)
		if len(index) > indexWidth {
			indexWidth = len(index)
		}
		for c, column := range columns {
			cell := ""
			if entry, found := row.GetOwnEntry(value.EncodeHandle(heap.Allocate(NewLightString(column)))); found {
				cell = String(entry.Value)
			}
			cellValues[r][c] = cell
			if len(cell) > widths[c] {
				widths[c] = len(cell)
			}
		}
	}
	line := func(left, middle, right string) string {
		var b strings.Builder
		b.WriteString(left)
		b.WriteString(strings.Repeat("─", indexWidth+2))
		for _, width := range widths {
			b.WriteString(middle)
			b.WriteString(strings.Repeat("─", width+2))
		}
		b.WriteString(right)
		b.WriteString("\n")
		return b.String()
	}
	var b strings.Builder
	b.WriteString(line("┌", "┬", "┐"))
	b.WriteString("│ ")
	b.WriteString(fmt.Sprintf("%-*s", indexWidth, "(index)"))
	b.WriteString(" ")
	for i, column := range columns {
		b.WriteString("│ ")
		b.WriteString(fmt.Sprintf("%-*s", widths[i], column))
		b.WriteString(" ")
	}
	b.WriteString("│\n")
	b.WriteString(line("├", "┼", "┤"))
	for r, row := range cellValues {
		b.WriteString("│ ")
		b.WriteString(fmt.Sprintf("%-*s", indexWidth, strconv.Itoa(r)))
		b.WriteString(" ")
		for c, cell := range row {
			b.WriteString("│ ")
			b.WriteString(fmt.Sprintf("%-*s", widths[c], cell))
			b.WriteString(" ")
		}
		b.WriteString("│\n")
	}
	b.WriteString(line("└", "┴", "┘"))
	if output != nil {
		output.WriteString(b.String())
	} else {
		fmt.Print(b.String())
	}
}

type Log struct {
	ObjNativeFn
}

func NewLog() *Log {
	log := &Log{}
	log.name = "log"
	return log
}

func consoleLogString(v value.Value) string {
	if v.IsNumber() {
		n := v.AsNumber()
		if n == 0 && math.Signbit(n) {
			return "-0"
		}
	}
	result := String(v)
	if v.IsObject() {
		result = strings.TrimSuffix(result, "\n")
	}
	return result
}

func formatConsoleValues(values []value.Value) string {
	if len(values) == 0 {
		return ""
	}
	format, isFormat := ObjectStringValue(values[0])
	if !isFormat {
		parts := make([]string, len(values))
		for i, v := range values {
			parts[i] = consoleLogString(v)
		}
		return strings.Join(parts, " ")
	}
	var builder strings.Builder
	next := 1
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			builder.WriteByte(format[i])
			continue
		}
		directive := format[i+1]
		if directive == '%' {
			builder.WriteByte('%')
			i++
			continue
		}
		if directive != 's' && directive != 'd' && directive != 'i' && directive != 'f' && directive != 'o' && directive != 'O' && directive != 'j' && directive != 'c' {
			builder.WriteByte(format[i])
			continue
		}
		if next >= len(values) {
			builder.WriteByte(format[i])
			continue
		}
		arg := values[next]
		next++
		switch directive {
		case 'c':
		case 'd', 'i', 'f':
			if arg.IsNumber() {
				builder.WriteString(nativeNumberString(arg.AsNumber()))
			} else {
				builder.WriteString(consoleLogString(arg))
			}
		default:
			builder.WriteString(consoleLogString(arg))
		}
		i++
	}
	for ; next < len(values); next++ {
		builder.WriteByte(' ')
		builder.WriteString(consoleLogString(values[next]))
	}
	return builder.String()
}

func (*Log) Log(values []value.Value, sb *strings.Builder) {
	text := formatConsoleValues(values)
	if sb != nil {
		fmt.Fprintln(sb, text)
	} else {
		fmt.Println(text)
	}
}

func LogStderr(values []value.Value) {
	fmt.Fprintln(os.Stderr, formatConsoleValues(values))
}
