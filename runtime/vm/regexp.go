package vm

import (
	"strings"
	"unicode/utf16"

	"github.com/dlclark/regexp2"

	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
)

func translateRegExpPattern(pattern string) string {
	pattern = strings.ReplaceAll(pattern, `\/`, "/")
	aliases := map[string]string{
		`\p{Letter}`: `\p{L}`,
		`\p{Number}`: `\p{N}`,
		`\P{Letter}`: `\P{L}`,
		`\P{Number}`: `\P{N}`,
	}
	for from, to := range aliases {
		pattern = strings.ReplaceAll(pattern, from, to)
	}
	return pattern
}

type regexpCapture struct {
	name string
}

func parseRegExpCaptures(pattern string) []regexpCapture {
	captures := []regexpCapture{}
	inClass := false
	escaped := false
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if inClass {
			if ch == ']' {
				inClass = false
			}
			continue
		}
		if ch == '[' {
			inClass = true
			continue
		}
		if ch != '(' {
			continue
		}
		if i+1 >= len(pattern) || pattern[i+1] != '?' {
			captures = append(captures, regexpCapture{})
			continue
		}
		if i+2 >= len(pattern) {
			continue
		}
		switch pattern[i+2] {
		case ':':
		case '=', '!':
		case '<':
			if i+3 < len(pattern) && (pattern[i+3] == '=' || pattern[i+3] == '!') {
				continue
			}
			end := strings.IndexByte(pattern[i+3:], '>')
			if end >= 0 {
				name := pattern[i+3 : i+3+end]
				captures = append(captures, regexpCapture{name: name})
			}
		}
	}
	return captures
}

func (vm *VM) compileRegExp(pattern, flags string) (*native.ObjRegExp, error) {
	seen := map[rune]bool{}
	hasUnicode := false
	hasUnicodeSets := false
	for _, flag := range flags {
		switch flag {
		case 'g', 'i', 'm', 's', 'u', 'y', 'd':
		case 'v':
			hasUnicodeSets = true
		default:
			return nil, &JSException{Name: "SyntaxError", Message: "Invalid regular expression flags"}
		}
		if seen[flag] {
			return nil, &JSException{Name: "SyntaxError", Message: "Invalid regular expression flags"}
		}
		seen[flag] = true
		if flag == 'u' {
			hasUnicode = true
		}
	}
	if hasUnicode && hasUnicodeSets {
		return nil, &JSException{Name: "SyntaxError", Message: "Invalid regular expression flags"}
	}
	options := regexp2.RegexOptions(regexp2.ECMAScript)
	if seen['i'] {
		options |= regexp2.IgnoreCase
	}
	if seen['m'] {
		options |= regexp2.Multiline
	}
	if seen['s'] {
		options |= regexp2.Singleline
	}
	if hasUnicode || hasUnicodeSets {
		options |= regexp2.Unicode
	}
	compiled, err := regexp2.Compile(translateRegExpPattern(pattern), options)
	if err != nil {
		return nil, &JSException{Name: "SyntaxError", Message: err.Error()}
	}
	obj := native.NewRegExpObject(pattern, flags)
	obj.Compiled = compiled
	unnamed := 0
	for _, capture := range parseRegExpCaptures(pattern) {
		groupNumber := 0
		if capture.name == "" {
			unnamed++
			groupNumber = unnamed
		} else {
			groupNumber = compiled.GroupNumberFromName(capture.name)
			obj.NamedGroups = append(obj.NamedGroups, capture.name)
		}
		if groupNumber > 0 {
			obj.GroupOrder = append(obj.GroupOrder, groupNumber)
		}
	}
	return obj, nil
}

func (vm *VM) constructRegExp(args []value.Value, clone bool) (value.Value, error) {
	pattern := ""
	flags := ""
	if len(args) > 0 {
		if existing, ok := vm.regexpFromValue(args[0]); ok {
			pattern = existing.Pattern
			if len(args) > 1 && args[1] != value.UNDEFINED {
				flagValue, err := vm.toStringValue(args[1])
				if err != nil {
					return value.UNDEFINED, err
				}
				flags, _ = vm.valueAsString(flagValue)
			} else {
				flags = existing.Flags
			}
			if !clone && len(args) < 2 {
				return args[0], nil
			}
		} else {
			patternValue, err := vm.toStringValue(args[0])
			if err != nil {
				return value.UNDEFINED, err
			}
			pattern, _ = vm.valueAsString(patternValue)
			if len(args) > 1 && args[1] != value.UNDEFINED {
				flagValue, err := vm.toStringValue(args[1])
				if err != nil {
					return value.UNDEFINED, err
				}
				flags, _ = vm.valueAsString(flagValue)
			}
		}
	}
	obj, err := vm.compileRegExp(pattern, flags)
	if err != nil {
		return value.UNDEFINED, err
	}
	return value.EncodeHandle(heap.Allocate(obj)), nil
}

func (vm *VM) regexpFromValue(v value.Value) (*native.ObjRegExp, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	re, ok := obj.(*native.ObjRegExp)
	return re, ok
}

type regexpText struct {
	original string
	units    []rune
	unicode  bool
}

func newRegexpText(text string, unicode bool) regexpText {
	if unicode {
		return regexpText{original: text, units: []rune(text), unicode: true}
	}
	units16 := utf16.Encode([]rune(text))
	units := make([]rune, len(units16))
	for i, unit := range units16 {
		units[i] = rune(unit)
	}
	return regexpText{original: text, units: units}
}

func (t regexpText) utf16Index(runeIndex int) int {
	if !t.unicode {
		return runeIndex
	}
	if runeIndex <= 0 {
		return 0
	}
	if runeIndex > len(t.units) {
		runeIndex = len(t.units)
	}
	return native.UTF16Length(string(t.units[:runeIndex]))
}

func (t regexpText) runeIndex(utf16Index int) int {
	if !t.unicode {
		return utf16Index
	}
	if utf16Index <= 0 {
		return 0
	}
	units := native.UTF16Units(t.original)
	if utf16Index > len(units) {
		utf16Index = len(units)
	}
	return len([]rune(native.UTF16FromUnits(units[:utf16Index])))
}

func (t regexpText) groupValue(group *regexp2.Group) value.Value {
	if group == nil || len(group.Captures) == 0 {
		return value.UNDEFINED
	}
	if t.unicode {
		return value.EncodeHandle(heap.Allocate(native.NewLightString(group.String())))
	}
	units := make([]uint16, group.Length)
	for i := 0; i < group.Length; i++ {
		units[i] = uint16(t.units[group.Index+i])
	}
	return value.EncodeHandle(heap.Allocate(native.NewLightString(native.UTF16FromUnits(units))))
}

func (t regexpText) groupLength(group *regexp2.Group) int {
	if group == nil || len(group.Captures) == 0 {
		return 0
	}
	if t.unicode {
		return native.UTF16Length(group.String())
	}
	return group.Length
}

func (vm *VM) regexpFind(re *native.ObjRegExp, text regexpText, start int, sticky bool) (*regexp2.Match, error) {
	startRune := text.runeIndex(start)
	match, err := re.Compiled.FindRunesMatchStartingAt(text.units, startRune)
	if err != nil {
		return nil, &JSException{Name: "Error", Message: err.Error()}
	}
	if match == nil {
		return nil, nil
	}
	if sticky && match.Index != startRune {
		return nil, nil
	}
	return match, nil
}

func (vm *VM) regexpResult(re *native.ObjRegExp, text regexpText, match *regexp2.Match) value.Value {
	if match == nil {
		return value.NULL
	}
	groups := []*regexp2.Group{}
	if len(re.GroupOrder) > 0 {
		groups = append(groups, &match.Group)
		for _, groupNumber := range re.GroupOrder {
			groups = append(groups, match.GroupByNumber(groupNumber))
		}
	} else {
		matchGroups := match.Groups()
		for i := range matchGroups {
			group := matchGroups[i]
			groups = append(groups, &group)
		}
	}
	items := make([]value.Value, len(groups))
	for i, group := range groups {
		items[i] = text.groupValue(group)
	}
	arr := native.NewArrayFrom(items)
	arr.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("index"))), value.ValueFromFloat64(float64(text.utf16Index(match.Index))))
	arr.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("input"))), value.EncodeHandle(heap.Allocate(native.NewLightString(text.original))))

	if len(re.NamedGroups) > 0 {
		named := native.NewNullPrototypeObject()
		for _, name := range re.NamedGroups {
			group := match.GroupByName(name)
			named.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString(name))), text.groupValue(group))
		}
		arr.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("groups"))), value.EncodeHandle(heap.Allocate(named)))
	}

	if strings.Contains(re.Flags, "d") {
		indices := make([]value.Value, len(groups))
		for i, group := range groups {
			if group == nil || len(group.Captures) == 0 {
				indices[i] = value.UNDEFINED
				continue
			}
			start := text.utf16Index(group.Index)
			end := start + text.groupLength(group)
			indices[i] = value.EncodeHandle(heap.Allocate(native.NewArrayFrom([]value.Value{value.ValueFromFloat64(float64(start)), value.ValueFromFloat64(float64(end))})))
		}
		indicesArr := native.NewArrayFrom(indices)
		if len(re.NamedGroups) > 0 {
			named := native.NewNullPrototypeObject()
			for _, name := range re.NamedGroups {
				group := match.GroupByName(name)
				if group == nil || len(group.Captures) == 0 {
					named.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString(name))), value.UNDEFINED)
					continue
				}
				start := text.utf16Index(group.Index)
				end := start + text.groupLength(group)
				named.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString(name))), value.EncodeHandle(heap.Allocate(native.NewArrayFrom([]value.Value{value.ValueFromFloat64(float64(start)), value.ValueFromFloat64(float64(end))}))))
			}
			indicesArr.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("groups"))), value.EncodeHandle(heap.Allocate(named)))
		}
		arr.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("indices"))), value.EncodeHandle(heap.Allocate(indicesArr)))
	}
	return value.EncodeHandle(heap.Allocate(arr))
}

func (vm *VM) callRegExpMethod(re *native.ObjRegExp, kind string, argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	textString := ""
	if len(args) > 0 && args[0] != value.UNDEFINED {
		textValue, err := vm.toStringValue(args[0])
		if err != nil {
			return value.UNDEFINED, err
		}
		textString, _ = vm.valueAsString(textValue)
	}
	text := newRegexpText(textString, re.Unicode)
	start := 0
	global := strings.Contains(re.Flags, "g")
	sticky := strings.Contains(re.Flags, "y")
	if global || sticky {
		start = re.LastIndex
		if start < 0 {
			start = 0
			re.LastIndex = 0
		} else if start > native.UTF16Length(textString) {
			re.LastIndex = 0
			if kind == "test" {
				return value.FALSE, nil
			}
			return value.NULL, nil
		}
	}
	match, err := vm.regexpFind(re, text, start, sticky)
	if err != nil {
		return value.UNDEFINED, err
	}
	if global || sticky {
		if match == nil {
			re.LastIndex = 0
		} else {
			re.LastIndex = text.utf16Index(match.Index + match.Length)
		}
	}
	if kind == "test" {
		return boolValue(match != nil), nil
	}
	return vm.regexpResult(re, text, match), nil
}

func (vm *VM) regexpObjectType(v value.Value) (object.Object, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	return obj, err == nil
}
