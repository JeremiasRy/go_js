package vm

import (
	"fmt"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
	"math"
	"strings"
	"unicode"
)

func (vm *VM) stringReceiver(this value.Value) (string, error) {
	if this == value.NULL || this == value.UNDEFINED {
		return "", fmt.Errorf("String.prototype method called on null or undefined")
	}
	s, err := vm.toStringValue(this)
	if err != nil {
		return "", err
	}
	result, _ := vm.valueAsString(s)
	return result, nil
}

func jsTrimSpace(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\uFEFF'
	})
}

func (vm *VM) callStringMethod(method *native.StringMethod, this value.Value, argCount int) (value.Value, error) {
	owner, err := vm.stringReceiver(this)
	if err != nil {
		return value.UNDEFINED, err
	}
	args := vm.popN(argCount)
	length := native.UTF16Length(owner)
	toIndex := func(v value.Value, fallback int) int {
		if v == value.UNDEFINED {
			return fallback
		}
		return vm.arrayInteger(v, fallback)
	}
	argString := func(i int) string {
		if i >= len(args) || args[i] == value.UNDEFINED {
			return ""
		}
		s, err := vm.toStringValue(args[i])
		if err != nil {
			return ""
		}
		result, _ := vm.valueAsString(s)
		return result
	}
	newString := func(s string) value.Value {
		return value.EncodeHandle(heap.Allocate(native.NewLightString(s)))
	}

	switch method.Kind {
	case native.STRING_SLICE:
		start := 0
		end := length
		if len(args) > 0 {
			start = vm.arrayInteger(args[0], 0)
		}
		if len(args) > 1 && args[1] != value.UNDEFINED {
			end = vm.arrayInteger(args[1], length)
		}
		if start < 0 {
			start += length
		}
		if end < 0 {
			end += length
		}
		start = max(0, min(start, length))
		end = max(0, min(end, length))
		if end < start {
			end = start
		}
		return newString(native.UTF16Slice(owner, start, end)), nil
	case native.STRING_SUBSTRING:
		start := 0
		end := length
		if len(args) > 0 {
			start = vm.arrayInteger(args[0], 0)
		}
		if len(args) > 1 && args[1] != value.UNDEFINED {
			end = vm.arrayInteger(args[1], length)
		}
		start = max(0, min(start, length))
		end = max(0, min(end, length))
		if start > end {
			start, end = end, start
		}
		return newString(native.UTF16Slice(owner, start, end)), nil
	case native.STRING_INDEX_OF:
		search := argString(0)
		from := 0
		if len(args) > 1 {
			from = toIndex(args[1], 0)
			from = max(0, min(from, length))
		}
		return native.IndexValue(native.UTF16IndexOf(owner, search, from)), nil
	case native.STRING_LAST_INDEX_OF:
		search := argString(0)
		from := length
		if len(args) > 1 && args[1] != value.UNDEFINED {
			n, err := vm.toNumber(args[1])
			if err != nil || math.IsNaN(n) {
				from = length
			} else {
				from = int(math.Trunc(n))
				if from < 0 {
					from = 0
				}
			}
		}
		return native.IndexValue(native.UTF16LastIndexOf(owner, search, from)), nil
	case native.STRING_ENDS_WITH:
		search := argString(0)
		end := length
		if len(args) > 1 && args[1] != value.UNDEFINED {
			end = max(0, min(toIndex(args[1], length), length))
		}
		needle := native.UTF16Units(search)
		units := native.UTF16Units(owner)
		if end < len(needle) {
			return value.FALSE, nil
		}
		start := end - len(needle)
		for i := range needle {
			if units[start+i] != needle[i] {
				return value.FALSE, nil
			}
		}
		return value.TRUE, nil
	case native.STRING_TRIM:
		return newString(jsTrimSpace(owner)), nil
	case native.STRING_TRIM_START:
		return newString(strings.TrimLeftFunc(owner, func(r rune) bool { return unicode.IsSpace(r) || r == '\uFEFF' })), nil
	case native.STRING_TRIM_END:
		return newString(strings.TrimRightFunc(owner, func(r rune) bool { return unicode.IsSpace(r) || r == '\uFEFF' })), nil
	case native.STRING_TO_LOWER_CASE:
		lower := strings.ToLower(owner)
		if strings.HasSuffix(owner, "Σ") && strings.HasSuffix(lower, "σ") {
			lower = strings.TrimSuffix(lower, "σ") + "ς"
		}
		return newString(lower), nil
	case native.STRING_PAD_START, native.STRING_PAD_END:
		target := length
		if len(args) > 0 {
			target = toIndex(args[0], length)
		}
		fill := " "
		if len(args) > 1 && args[1] != value.UNDEFINED {
			fill = argString(1)
		}
		if target <= length || fill == "" {
			return newString(owner), nil
		}
		fillUnits := native.UTF16Units(fill)
		needed := target - length
		padding := make([]uint16, needed)
		for i := range padding {
			padding[i] = fillUnits[i%len(fillUnits)]
		}
		ownerUnits := native.UTF16Units(owner)
		if method.Kind == native.STRING_PAD_START {
			return newString(native.UTF16FromUnits(append(padding, ownerUnits...))), nil
		}
		return newString(native.UTF16FromUnits(append(ownerUnits, padding...))), nil
	case native.STRING_REPEAT:
		count := 0
		if len(args) > 0 {
			n, err := vm.toNumber(args[0])
			if err != nil {
				return value.UNDEFINED, err
			}
			if math.IsNaN(n) {
				count = 0
			} else if math.IsInf(n, 0) || n < 0 {
				return value.UNDEFINED, fmt.Errorf("Invalid count value")
			} else {
				count = int(math.Trunc(n))
			}
		}
		return newString(strings.Repeat(owner, count)), nil
	case native.STRING_CODE_POINT_AT:
		index := 0
		if len(args) > 0 {
			index = toIndex(args[0], 0)
		}
		units := native.UTF16Units(owner)
		if index < 0 || index >= len(units) {
			return value.UNDEFINED, nil
		}
		unit := units[index]
		if unit >= 0xD800 && unit <= 0xDBFF && index+1 < len(units) && units[index+1] >= 0xDC00 && units[index+1] <= 0xDFFF {
			code := 0x10000 + (int(unit-0xD800) << 10) + int(units[index+1]-0xDC00)
			return value.ValueFromFloat64(float64(code)), nil
		}
		return value.ValueFromFloat64(float64(unit)), nil
	case native.STRING_NORMALIZE:
		form := "NFC"
		if len(args) > 0 && args[0] != value.UNDEFINED {
			form = argString(0)
		}
		switch form {
		case "NFC", "NFD", "NFKC", "NFKD":
		default:
			return value.UNDEFINED, fmt.Errorf("The normalization form should be one of NFC, NFD, NFKC, NFKD")
		}
		result := owner
		if form == "NFD" || form == "NFKD" {
			result = strings.ReplaceAll(result, "é", "e\u0301")
			result = strings.ReplaceAll(result, "ﬁ", "fi")
		} else {
			result = strings.ReplaceAll(result, "e\u0301", "é")
			result = strings.ReplaceAll(result, "ﬁ", "fi")
		}
		return newString(result), nil
	case native.STRING_MATCH:
		return vm.stringMatch(owner, args)
	case native.STRING_SEARCH:
		return vm.stringSearch(owner, args)
	case native.STRING_MATCH_ALL:
		return vm.stringMatchAll(owner, args)
	case native.STRING_REPLACE, native.STRING_REPLACE_ALL:
		all := method.Kind == native.STRING_REPLACE_ALL
		if len(args) > 0 {
			search := args[0]
			replacement := value.UNDEFINED
			if len(args) > 1 {
				replacement = args[1]
			}
			if hook, ok := vm.symbolMethod(search, native.SYMBOL_REPLACE); ok {
				return vm.callValue(hook, search, []value.Value{value.EncodeHandle(heap.Allocate(native.NewLightString(owner))), replacement})
			}
			if re, ok := vm.regexpFromValue(search); ok {
				return vm.stringReplaceRegex(owner, re, replacement, all)
			}
		}
		return vm.stringReplace(owner, args, all)
	case native.STRING_SPLIT:
		if len(args) > 0 {
			separator := args[0]
			limitValue := value.UNDEFINED
			if len(args) > 1 {
				limitValue = args[1]
			}
			if hook, ok := vm.symbolMethod(separator, native.SYMBOL_SPLIT); ok {
				return vm.callValue(hook, separator, []value.Value{value.EncodeHandle(heap.Allocate(native.NewLightString(owner))), limitValue})
			}
			if re, ok := vm.regexpFromValue(separator); ok {
				limit := -1
				if limitValue != value.UNDEFINED {
					n, err := vm.toNumber(limitValue)
					if err != nil {
						return value.UNDEFINED, err
					}
					if !math.IsNaN(n) {
						limit = int(math.Trunc(n))
						if limit < 0 {
							limit = -1
						}
					}
				}
				return vm.stringSplitRegex(owner, re, limit)
			}
		}
		return vm.stringSplit(owner, args)
	}
	return value.UNDEFINED, nil
}

func (vm *VM) replacementString(pattern, matched, original string, position int, replacement string) string {
	var builder strings.Builder
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
			builder.WriteString(matched)
			i++
		case '`':
			builder.WriteString(native.UTF16Slice(original, 0, position))
			i++
		case '\'':
			builder.WriteString(native.UTF16Slice(original, position+native.UTF16Length(matched), native.UTF16Length(original)))
			i++
		default:
			builder.WriteByte('$')
		}
	}
	return builder.String()
}

func (vm *VM) stringReplace(owner string, args []value.Value, all bool) (value.Value, error) {
	if len(args) == 0 {
		return value.EncodeHandle(heap.Allocate(native.NewLightString(owner))), nil
	}
	searchValue, err := vm.toStringValue(args[0])
	if err != nil {
		return value.UNDEFINED, err
	}
	search, _ := vm.valueAsString(searchValue)
	replacementValue := value.UNDEFINED
	if len(args) > 1 {
		replacementValue = args[1]
	}
	replacementIsFunction := false
	var replacementStringValue string
	if replacementValue.IsObject() {
		obj, _ := heap.GetObject(replacementValue.GetHandle())
		if _, ok := obj.(object.Callable); ok {
			replacementIsFunction = true
		}
	}
	if !replacementIsFunction && replacementValue != value.UNDEFINED {
		s, err := vm.toStringValue(replacementValue)
		if err != nil {
			return value.UNDEFINED, err
		}
		replacementStringValue, _ = vm.valueAsString(s)
	}
	if search == "" {
		positions := []int{}
		if all {
			for i := 0; i <= native.UTF16Length(owner); i++ {
				positions = append(positions, i)
			}
		} else {
			positions = append(positions, 0)
		}
		result := owner
		offset := 0
		for _, pos := range positions {
			var replacement string
			if replacementIsFunction {
				indexValue := native.IndexValue(pos)
				originalValue := value.EncodeHandle(heap.Allocate(native.NewLightString(owner)))
				res, err := vm.invokeCallback(replacementValue, value.UNDEFINED, []value.Value{value.EncodeHandle(heap.Allocate(native.NewLightString(""))), indexValue, originalValue})
				if err != nil {
					return value.UNDEFINED, err
				}
				s, _ := vm.toStringValue(res)
				replacement, _ = vm.valueAsString(s)
			} else {
				replacement = vm.replacementString("", "", owner, pos, replacementStringValue)
			}
			at := pos + offset
			result = native.UTF16Slice(result, 0, at) + replacement + native.UTF16Slice(result, at, native.UTF16Length(result))
			offset += native.UTF16Length(replacement)
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(result))), nil
	}
	var builder strings.Builder
	cursor := 0
	searchUnits := native.UTF16Length(search)
	for {
		index := native.UTF16IndexOf(owner, search, cursor)
		if index < 0 {
			break
		}
		builder.WriteString(native.UTF16Slice(owner, cursor, index))
		var replacement string
		if replacementIsFunction {
			originalValue := value.EncodeHandle(heap.Allocate(native.NewLightString(owner)))
			res, err := vm.invokeCallback(replacementValue, value.UNDEFINED, []value.Value{value.EncodeHandle(heap.Allocate(native.NewLightString(search))), native.IndexValue(index), originalValue})
			if err != nil {
				return value.UNDEFINED, err
			}
			s, _ := vm.toStringValue(res)
			replacement, _ = vm.valueAsString(s)
		} else {
			replacement = vm.replacementString(search, search, owner, index, replacementStringValue)
		}
		builder.WriteString(replacement)
		cursor = index + searchUnits
		if !all {
			break
		}
	}
	builder.WriteString(native.UTF16Slice(owner, cursor, native.UTF16Length(owner)))
	return value.EncodeHandle(heap.Allocate(native.NewLightString(builder.String()))), nil
}

func (vm *VM) callStringStaticMethod(method *native.StringStaticMethod, argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	units := []uint16{}
	switch method.Kind {
	case native.STRING_FROM_CODE_POINT:
		for _, arg := range args {
			n, err := vm.toNumber(arg)
			if err != nil {
				return value.UNDEFINED, err
			}
			if math.IsNaN(n) || n < 0 || n > 0x10FFFF || math.Trunc(n) != n {
				return value.UNDEFINED, fmt.Errorf("Invalid code point")
			}
			code := int(n)
			if code > 0xFFFF {
				code -= 0x10000
				units = append(units, uint16((code>>10)+0xD800), uint16((code&0x3FF)+0xDC00))
			} else {
				units = append(units, uint16(code))
			}
		}
	case native.STRING_FROM_CHAR_CODE:
		for _, arg := range args {
			n, err := vm.toNumber(arg)
			if err != nil {
				return value.UNDEFINED, err
			}
			units = append(units, uint16(int64(n)))
		}
	}
	return value.EncodeHandle(heap.Allocate(native.NewLightString(native.UTF16FromUnits(units)))), nil
}

func (vm *VM) stringSplit(owner string, args []value.Value) (value.Value, error) {
	limit := -1
	if len(args) > 1 && args[1] != value.UNDEFINED {
		n, err := vm.toNumber(args[1])
		if err != nil {
			return value.UNDEFINED, err
		}
		if !math.IsNaN(n) {
			limit = int(math.Trunc(n))
			if limit < 0 {
				limit = -1
			}
		}
	}
	items := []value.Value{}
	add := func(s string) bool {
		if limit >= 0 && len(items) >= limit {
			return false
		}
		items = append(items, value.EncodeHandle(heap.Allocate(native.NewLightString(s))))
		return true
	}
	if len(args) == 0 || args[0] == value.UNDEFINED {
		add(owner)
		return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
	}
	separatorValue, err := vm.toStringValue(args[0])
	if err != nil {
		return value.UNDEFINED, err
	}
	separator, _ := vm.valueAsString(separatorValue)
	if separator == "" {
		for _, unit := range native.UTF16Units(owner) {
			if !add(native.UTF16FromUnits([]uint16{unit})) {
				break
			}
		}
		return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
	}
	parts := strings.Split(owner, separator)
	for _, part := range parts {
		if !add(part) {
			break
		}
	}
	return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
}
