package vm

import (
	"strings"

	"github.com/dlclark/regexp2"

	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
)

func (vm *VM) symbolMethod(target value.Value, symbol value.Value) (value.Value, bool) {
	if symbol == 0 || !target.IsObject() {
		return value.UNDEFINED, false
	}
	method, err := vm.getMemberValue(target, symbol)
	if err != nil || method == value.UNDEFINED {
		return value.UNDEFINED, false
	}
	obj, err := heap.GetObject(method.GetHandle())
	if err != nil {
		return value.UNDEFINED, false
	}
	if _, ok := obj.(interface{ ValueChunk() *value.ValueChunk }); !ok {
		if obj.Type() != object.OBJ_NATIVE_FN {
			return value.UNDEFINED, false
		}
	}
	return method, true
}

func (vm *VM) regexpGroups(re *native.ObjRegExp, text regexpText, match *regexp2.Match) []value.Value {
	groups := []value.Value{}
	if len(re.GroupOrder) > 0 {
		groups = append(groups, text.groupValue(&match.Group))
		for _, groupNumber := range re.GroupOrder {
			groups = append(groups, text.groupValue(match.GroupByNumber(groupNumber)))
		}
		return groups
	}
	matchGroups := match.Groups()
	for i := range matchGroups {
		group := matchGroups[i]
		groups = append(groups, text.groupValue(&group))
	}
	return groups
}

func (vm *VM) regexpNamedGroups(re *native.ObjRegExp, text regexpText, match *regexp2.Match) value.Value {
	if len(re.NamedGroups) == 0 {
		return value.UNDEFINED
	}
	groups := native.NewNullPrototypeObject()
	for _, name := range re.NamedGroups {
		groups.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString(name))), text.groupValue(match.GroupByName(name)))
	}
	return value.EncodeHandle(heap.Allocate(groups))
}

func (vm *VM) stringMatch(owner string, args []value.Value) (value.Value, error) {
	if len(args) == 0 {
		re, _ := vm.compileRegExp("", "")
		return vm.regexpResult(re, newRegexpText(owner, false), mustMatch(vm, re, owner)), nil
	}
	search := args[0]
	if method, ok := vm.symbolMethod(search, native.SYMBOL_MATCH); ok {
		return vm.callValue(method, search, []value.Value{value.EncodeHandle(heap.Allocate(native.NewLightString(owner)))})
	}
	re, ok := vm.regexpFromValue(search)
	if !ok {
		compiled, err := vm.compileRegExp(mustString(vm, search), "")
		if err != nil {
			return value.UNDEFINED, err
		}
		re = compiled
	}
	text := newRegexpText(owner, re.Unicode)
	global := strings.Contains(re.Flags, "g")
	if !global {
		match, err := vm.regexpFind(re, text, 0, false)
		if err != nil {
			return value.UNDEFINED, err
		}
		return vm.regexpResult(re, text, match), nil
	}
	results := []value.Value{}
	start := 0
	for start < native.UTF16Length(owner) {
		match, err := vm.regexpFind(re, text, start, false)
		if err != nil {
			return value.UNDEFINED, err
		}
		if match == nil {
			break
		}
		results = append(results, text.groupValue(&match.Group))
		end := text.utf16Index(match.Index + match.Length)
		if match.Length == 0 {
			end = advanceStringIndex(owner, end, re.Unicode)
		}
		if end <= start {
			end = start + 1
		}
		start = end
	}
	re.LastIndex = 0
	if len(results) == 0 {
		return value.NULL, nil
	}
	return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(results))), nil
}

func mustMatch(vm *VM, re *native.ObjRegExp, text string) *regexp2.Match {
	match, _ := vm.regexpFind(re, newRegexpText(text, re.Unicode), 0, false)
	return match
}

func mustString(vm *VM, v value.Value) string {
	s, err := vm.toStringValue(v)
	if err != nil {
		return ""
	}
	result, _ := vm.valueAsString(s)
	return result
}

func advanceStringIndex(text string, index int, unicode bool) int {
	if !unicode {
		return index + 1
	}
	units := native.UTF16Units(text)
	if index+1 < len(units) && units[index] >= 0xD800 && units[index] <= 0xDBFF && units[index+1] >= 0xDC00 && units[index+1] <= 0xDFFF {
		return index + 2
	}
	return index + 1
}

func (vm *VM) stringSearch(owner string, args []value.Value) (value.Value, error) {
	search := value.UNDEFINED
	if len(args) > 0 {
		search = args[0]
	}
	if method, ok := vm.symbolMethod(search, native.SYMBOL_SEARCH); ok {
		return vm.callValue(method, search, []value.Value{value.EncodeHandle(heap.Allocate(native.NewLightString(owner)))})
	}
	re, ok := vm.regexpFromValue(search)
	if !ok {
		return native.IndexValue(native.UTF16IndexOf(owner, mustString(vm, search), 0)), nil
	}
	saved := re.LastIndex
	match, err := vm.regexpFind(re, newRegexpText(owner, re.Unicode), 0, false)
	re.LastIndex = saved
	if err != nil {
		return value.UNDEFINED, err
	}
	if match == nil {
		return value.ValueFromFloat64(-1), nil
	}
	return value.ValueFromFloat64(float64(newRegexpText(owner, re.Unicode).utf16Index(match.Index))), nil
}

func (vm *VM) stringMatchAll(owner string, args []value.Value) (value.Value, error) {
	search := value.UNDEFINED
	if len(args) > 0 {
		search = args[0]
	}
	if method, ok := vm.symbolMethod(search, native.SYMBOL_MATCH_ALL); ok {
		return vm.callValue(method, search, []value.Value{value.EncodeHandle(heap.Allocate(native.NewLightString(owner)))})
	}
	re, ok := vm.regexpFromValue(search)
	if !ok || !strings.Contains(re.Flags, "g") {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "String.prototype.matchAll called with a non-global RegExp argument"}
	}
	clone, err := vm.compileRegExp(re.Pattern, re.Flags)
	if err != nil {
		return value.UNDEFINED, err
	}
	text := newRegexpText(owner, clone.Unicode)
	results := []value.Value{}
	start := re.LastIndex
	for start < native.UTF16Length(owner) {
		match, err := vm.regexpFind(clone, text, start, false)
		if err != nil {
			return value.UNDEFINED, err
		}
		if match == nil {
			break
		}
		results = append(results, vm.regexpResult(clone, text, match))
		end := text.utf16Index(match.Index + match.Length)
		if match.Length == 0 {
			end = advanceStringIndex(owner, end, clone.Unicode)
		}
		if end <= start {
			end = start + 1
		}
		start = end
	}
	return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(results))), nil
}

func (vm *VM) expandReplacement(re *native.ObjRegExp, text regexpText, match *regexp2.Match, replacement string) string {
	var builder strings.Builder
	matchStart := text.utf16Index(match.Index)
	matchEnd := matchStart + text.groupLength(&match.Group)
	groups := vm.regexpGroups(re, text, match)
	for i := 0; i < len(replacement); i++ {
		if replacement[i] != '$' || i+1 >= len(replacement) {
			builder.WriteByte(replacement[i])
			continue
		}
		next := replacement[i+1]
		switch next {
		case '$':
			builder.WriteByte('$')
			i++
		case '&':
			builder.WriteString(mustString(vm, text.groupValue(&match.Group)))
			i++
		case '`':
			builder.WriteString(native.UTF16Slice(text.original, 0, matchStart))
			i++
		case '\'':
			builder.WriteString(native.UTF16Slice(text.original, matchEnd, native.UTF16Length(text.original)))
			i++
		case '<':
			end := strings.IndexByte(replacement[i+2:], '>')
			if end < 0 {
				builder.WriteByte('$')
				continue
			}
			name := replacement[i+2 : i+2+end]
			group := match.GroupByName(name)
			if group != nil && len(group.Captures) > 0 {
				builder.WriteString(mustString(vm, text.groupValue(group)))
			}
			i += 2 + end
		default:
			if next >= '0' && next <= '9' {
				number := int(next - '0')
				consumed := 1
				if i+2 < len(replacement) && replacement[i+2] >= '0' && replacement[i+2] <= '9' {
					candidate := number*10 + int(replacement[i+2]-'0')
					if candidate < len(groups) {
						number = candidate
						consumed = 2
					}
				}
				if number < len(groups) && groups[number] != value.UNDEFINED {
					builder.WriteString(mustString(vm, groups[number]))
				}
				i += consumed
			} else {
				builder.WriteByte('$')
			}
		}
	}
	return builder.String()
}

func (vm *VM) stringReplaceRegex(owner string, re *native.ObjRegExp, replacement value.Value, all bool) (value.Value, error) {
	if all && !strings.Contains(re.Flags, "g") {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "String.prototype.replaceAll called with a non-global RegExp argument"}
	}
	replaceAll := all || strings.Contains(re.Flags, "g")
	text := newRegexpText(owner, re.Unicode)
	replacementStringValue := ""
	replacementIsFunction := false
	if replacement.IsObject() {
		if obj, err := heap.GetObject(replacement.GetHandle()); err == nil {
			if _, ok := obj.(interface{ ValueChunk() *value.ValueChunk }); ok {
				replacementIsFunction = true
			}
		}
	}
	if !replacementIsFunction && replacement != value.UNDEFINED {
		replacementStringValue = mustString(vm, replacement)
	}
	var builder strings.Builder
	cursor := 0
	start := 0
	for start < native.UTF16Length(owner) {
		match, err := vm.regexpFind(re, text, start, false)
		if err != nil {
			return value.UNDEFINED, err
		}
		if match == nil {
			break
		}
		matchStart := text.utf16Index(match.Index)
		matchEnd := matchStart + text.groupLength(&match.Group)
		builder.WriteString(native.UTF16Slice(owner, cursor, matchStart))
		var replacementText string
		if replacementIsFunction {
			groups := vm.regexpGroups(re, text, match)
			callbackArgs := []value.Value{}
			callbackArgs = append(callbackArgs, groups...)
			callbackArgs = append(callbackArgs, value.ValueFromFloat64(float64(matchStart)), value.EncodeHandle(heap.Allocate(native.NewLightString(owner))))
			if named := vm.regexpNamedGroups(re, text, match); named != value.UNDEFINED {
				callbackArgs = append(callbackArgs, named)
			}
			result, err := vm.invokeCallback(replacement, value.UNDEFINED, callbackArgs)
			if err != nil {
				return value.UNDEFINED, err
			}
			replacementText = mustString(vm, result)
		} else {
			replacementText = vm.expandReplacement(re, text, match, replacementStringValue)
		}
		builder.WriteString(replacementText)
		cursor = matchEnd
		if !replaceAll {
			break
		}
		if match.Length == 0 {
			next := advanceStringIndex(owner, matchEnd, re.Unicode)
			if next > native.UTF16Length(owner) {
				break
			}
			if next > cursor {
				builder.WriteString(native.UTF16Slice(owner, cursor, next))
				cursor = next
			}
			start = next
		} else {
			start = matchEnd
		}
	}
	builder.WriteString(native.UTF16Slice(owner, cursor, native.UTF16Length(owner)))
	return value.EncodeHandle(heap.Allocate(native.NewLightString(builder.String()))), nil
}

func (vm *VM) stringSplitRegex(owner string, re *native.ObjRegExp, limit int) (value.Value, error) {
	text := newRegexpText(owner, re.Unicode)
	items := []value.Value{}
	add := func(v value.Value) bool {
		if limit >= 0 && len(items) >= limit {
			return false
		}
		items = append(items, v)
		return true
	}
	lastEnd := 0
	start := 0
	for start < native.UTF16Length(owner) {
		match, err := vm.regexpFind(re, text, start, false)
		if err != nil {
			return value.UNDEFINED, err
		}
		if match == nil {
			break
		}
		matchStart := text.utf16Index(match.Index)
		matchEnd := matchStart + text.groupLength(&match.Group)
		if matchStart == matchEnd && matchStart == lastEnd {
			start = advanceStringIndex(owner, matchEnd, re.Unicode)
			if start > native.UTF16Length(owner) {
				break
			}
			continue
		}
		if !add(value.EncodeHandle(heap.Allocate(native.NewLightString(native.UTF16Slice(owner, lastEnd, matchStart))))) {
			return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
		}
		groups := vm.regexpGroups(re, text, match)
		for i := 1; i < len(groups); i++ {
			if !add(groups[i]) {
				return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
			}
		}
		lastEnd = matchEnd
		if match.Length == 0 {
			start = advanceStringIndex(owner, matchEnd, re.Unicode)
		} else {
			start = matchEnd
		}
	}
	add(value.EncodeHandle(heap.Allocate(native.NewLightString(native.UTF16Slice(owner, lastEnd, native.UTF16Length(owner))))))
	return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
}

func (vm *VM) regexpSymbolMatch(re *native.ObjRegExp, owner string) (value.Value, error) {
	text := newRegexpText(owner, re.Unicode)
	if !strings.Contains(re.Flags, "g") {
		match, err := vm.regexpFind(re, text, 0, false)
		if err != nil {
			return value.UNDEFINED, err
		}
		return vm.regexpResult(re, text, match), nil
	}
	results := []value.Value{}
	start := 0
	for start < native.UTF16Length(owner) {
		match, err := vm.regexpFind(re, text, start, false)
		if err != nil {
			return value.UNDEFINED, err
		}
		if match == nil {
			break
		}
		results = append(results, text.groupValue(&match.Group))
		end := text.utf16Index(match.Index + match.Length)
		if match.Length == 0 {
			end = advanceStringIndex(owner, end, re.Unicode)
		}
		if end <= start {
			end = start + 1
		}
		start = end
	}
	re.LastIndex = 0
	if len(results) == 0 {
		return value.NULL, nil
	}
	return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(results))), nil
}
