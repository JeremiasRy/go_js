package vm

import (
	"regexp"
	"strings"

	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
)

func translateRegExpPattern(pattern string) string {
	pattern = strings.ReplaceAll(pattern, `\/`, "/")
	pattern = strings.ReplaceAll(pattern, `\u{`, `\x{`)
	return pattern
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
	prefix := ""
	if seen['i'] {
		prefix += "(?i)"
	}
	if seen['m'] {
		prefix += "(?m)"
	}
	if seen['s'] {
		prefix += "(?s)"
	}
	compiled, err := regexp.Compile(prefix + translateRegExpPattern(pattern))
	if err != nil {
		return nil, &JSException{Name: "SyntaxError", Message: err.Error()}
	}
	obj := native.NewRegExpObject(pattern, flags)
	obj.Compiled = compiled
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

func (vm *VM) regexpMatch(re *native.ObjRegExp, text string, start int, sticky bool) []int {
	if re.Compiled == nil {
		prefix := ""
		if strings.Contains(re.Flags, "i") {
			prefix += "(?i)"
		}
		if strings.Contains(re.Flags, "m") {
			prefix += "(?m)"
		}
		if strings.Contains(re.Flags, "s") {
			prefix += "(?s)"
		}
		compiled, err := regexp.Compile(prefix + translateRegExpPattern(re.Pattern))
		if err != nil {
			return nil
		}
		re.Compiled = compiled
	}
	if sticky {
		if start < 0 || start > len(text) {
			return nil
		}
		match := re.Compiled.FindStringSubmatchIndex(text[start:])
		if match == nil || match[0] != 0 {
			return nil
		}
		for i := range match {
			if match[i] >= 0 {
				match[i] += start
			}
		}
		return match
	}
	return re.Compiled.FindStringSubmatchIndex(text[start:])
}

func (vm *VM) regexpResult(re *native.ObjRegExp, text string, match []int, offset int) value.Value {
	if match == nil {
		return value.NULL
	}
	items := make([]value.Value, len(match)/2)
	for i := 0; i < len(match)/2; i++ {
		begin := match[i*2]
		end := match[i*2+1]
		if begin < 0 || end < 0 {
			items[i] = value.UNDEFINED
		} else {
			items[i] = value.EncodeHandle(heap.Allocate(native.NewLightString(text[begin+offset : end+offset])))
		}
	}
	arr := native.NewArrayFrom(items)
	arr.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("index"))), value.ValueFromFloat64(float64(match[0]+offset)))
	arr.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("input"))), value.EncodeHandle(heap.Allocate(native.NewLightString(text))))
	return value.EncodeHandle(heap.Allocate(arr))
}

func (vm *VM) callRegExpMethod(re *native.ObjRegExp, kind string, argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	text := ""
	if len(args) > 0 {
		textValue, err := vm.toStringValue(args[0])
		if err != nil {
			return value.UNDEFINED, err
		}
		text, _ = vm.valueAsString(textValue)
	}
	start := 0
	global := strings.Contains(re.Flags, "g")
	sticky := strings.Contains(re.Flags, "y")
	if global || sticky {
		start = re.LastIndex
		if start < 0 || start > len(text) {
			re.LastIndex = 0
			if kind == "test" {
				return value.FALSE, nil
			}
			return value.NULL, nil
		}
	}
	match := vm.regexpMatch(re, text, start, sticky)
	if global || sticky {
		if match == nil {
			re.LastIndex = 0
		} else {
			re.LastIndex = match[1]
		}
	}
	if kind == "test" {
		return boolValue(match != nil), nil
	}
	return vm.regexpResult(re, text, match, 0), nil
}

func (vm *VM) regexpObjectType(v value.Value) (object.Object, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	return obj, err == nil
}
