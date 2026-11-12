package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"math"
	"strconv"
	"strings"
	"time"
)

type Console struct {
	ObjObject
	timers map[string]time.Time
}

func (*Console) Type() object.ObjType {
	return object.OBJ_CONSOLE
}

func (*Console) String() string {
	return "Object [console]"
}

func NewObjectConsole() *Console {
	c := &Console{timers: map[string]time.Time{}}
	c.Members = map[string]ObjectValueEntry{}
	c.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	log := value.EncodeHandle(heap.Allocate(NewLightString("log")))

	c.SetMember(log, value.EncodeHandle(heap.Allocate(NewLog())))
	c.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("error"))), value.EncodeHandle(heap.Allocate(NewConsoleMethod("error", CONSOLE_ERROR))))
	c.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("table"))), value.EncodeHandle(heap.Allocate(NewConsoleMethod("table", CONSOLE_TABLE))))
	c.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("time"))), value.EncodeHandle(heap.Allocate(NewConsoleMethod("time", CONSOLE_TIME))))
	c.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("timeEnd"))), value.EncodeHandle(heap.Allocate(NewConsoleMethod("timeEnd", CONSOLE_TIME_END))))
	return c
}

type ConsoleMethodKind int

const (
	CONSOLE_ERROR ConsoleMethodKind = iota
	CONSOLE_TABLE
	CONSOLE_TIME
	CONSOLE_TIME_END
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
	return String(v)
}

func (*Log) Log(values []value.Value, sb *strings.Builder) {
	for i, v := range values {
		notLast := i < len(values)-1
		pad := ""
		if notLast {
			pad = " "
		}

		if sb != nil {
			fmt.Fprintf(sb, "%v%s", consoleLogString(v), pad)
		} else {
			fmt.Printf("%v%s", consoleLogString(v), pad)
		}
	}
	if sb != nil {
		fmt.Fprintln(sb)
	} else {
		fmt.Println()
	}
}
