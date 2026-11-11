package vm

import (
	"fmt"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
	"math"
	"strconv"
	"strings"
)

func jsonKey(k string) value.Value {
	return value.EncodeHandle(heap.Allocate(native.NewLightString(k)))
}

func (vm *VM) jsonRevive(holder value.Value, key string, reviver value.Value) (value.Value, error) {
	keyValue := jsonKey(key)
	val, err := vm.getMemberValue(holder, keyValue)
	if err != nil {
		return value.UNDEFINED, err
	}
	if val.IsObject() {
		obj, err := heap.GetObject(val.GetHandle())
		if err == nil {
			switch obj := obj.(type) {
			case *native.ObjArr:
				for i := 0; i < obj.Length(); i++ {
					child, err := vm.jsonRevive(val, strconv.Itoa(i), reviver)
					if err != nil {
						return value.UNDEFINED, err
					}
					if child == value.UNDEFINED {
						obj.DeleteIndex(i)
					} else {
						obj.SetElementAt(i, child)
					}
				}
			case *native.ObjObject:
				for _, item := range orderedEntriesForVM(obj) {
					if len(item.Key) > 0 && item.Key[0] == 0 {
						continue
					}
					child, err := vm.jsonRevive(val, item.Key, reviver)
					if err != nil {
						return value.UNDEFINED, err
					}
					if child == value.UNDEFINED {
						obj.Delete(jsonKey(item.Key))
					} else {
						obj.SetMember(jsonKey(item.Key), child)
					}
				}
			}
		}
	}
	result, err := vm.invokeCallback(reviver, holder, []value.Value{keyValue, val})
	if err != nil {
		return value.UNDEFINED, err
	}
	return result, nil
}

func (vm *VM) callJSONParse(argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	if len(args) == 0 {
		return value.UNDEFINED, fmt.Errorf("Unexpected token u in JSON")
	}
	textValue, err := vm.toStringValue(args[0])
	if err != nil {
		return value.UNDEFINED, err
	}
	text, _ := vm.valueAsString(textValue)
	parsed, err := native.JSONStringToValue(text)
	if err != nil {
		return value.UNDEFINED, err
	}
	if len(args) > 1 {
		reviver := args[1]
		if reviver.IsObject() {
			obj, _ := heap.GetObject(reviver.GetHandle())
			if _, ok := obj.(object.Callable); ok {
				holder := native.NewObjectHash()
				holder.SetMember(jsonKey(""), parsed)
				return vm.jsonRevive(value.EncodeHandle(heap.Allocate(holder)), "", reviver)
			}
		}
	}
	return parsed, nil
}

type jsonStringifier struct {
	vm        *VM
	replacer  value.Value
	propertyList []string
	space     string
	stack     map[uint32]bool
}

func (s *jsonStringifier) quote(str string) string {
	var builder strings.Builder
	builder.WriteByte('"')
	units := native.UTF16Units(str)
	for i := 0; i < len(units); i++ {
		unit := units[i]
		if unit >= 0xD800 && unit <= 0xDBFF && i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF {
			code := 0x10000 + (int(unit-0xD800) << 10) + int(units[i+1]-0xDC00)
			builder.WriteRune(rune(code))
			i++
			continue
		}
		switch unit {
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		case '\b':
			builder.WriteString(`\b`)
		case '\f':
			builder.WriteString(`\f`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		default:
			if unit < 0x20 || (unit >= 0xD800 && unit <= 0xDFFF) {
				builder.WriteString(fmt.Sprintf(`\u%04x`, unit))
			} else {
				builder.WriteString(native.UTF16FromUnits([]uint16{unit}))
			}
		}
	}
	builder.WriteByte('"')
	return builder.String()
}

func (s *jsonStringifier) serialize(key string, holder value.Value, indent string) (string, error) {
	val, err := s.vm.getMemberValue(holder, jsonKey(key))
	if err != nil {
		return "", err
	}
	if val.IsObject() {
		if obj, err := heap.GetObject(val.GetHandle()); err == nil {
			if hashable, ok := obj.(object.Hashable); ok {
				toJSON := hashable.GetMember(jsonKey("toJSON"))
				if toJSON.IsObject() {
					if toJSONObj, err := heap.GetObject(toJSON.GetHandle()); err == nil {
						if _, ok := toJSONObj.(object.Callable); ok {
							val, err = s.vm.invokeCallback(toJSON, val, []value.Value{jsonKey(key)})
							if err != nil {
								return "", err
							}
						}
					}
				}
			}
		}
	}
	if s.replacer.IsObject() {
		if obj, _ := heap.GetObject(s.replacer.GetHandle()); obj != nil {
			if _, ok := obj.(object.Callable); ok {
				val, err = s.vm.invokeCallback(s.replacer, holder, []value.Value{jsonKey(key), val})
				if err != nil {
					return "", err
				}
			}
		}
	}
	if val == value.UNDEFINED || val == value.UNINITIALIZED {
		return "", nil
	}
	if val.IsBoolean() {
		if val == value.TRUE {
			return "true", nil
		}
		return "false", nil
	}
	if val == value.NULL {
		return "null", nil
	}
	if val.IsNumber() {
		n := val.AsNumber()
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return "null", nil
		}
		if n == 0 {
			n = 0
		}
		return nativeNumberString(n), nil
	}
	if str, ok := s.vm.valueAsString(val); ok {
		return s.quote(str), nil
	}
	if !val.IsObject() {
		return "", nil
	}
	obj, err := heap.GetObject(val.GetHandle())
	if err != nil {
		return "", err
	}
	ptr := heap.GetPointer(val.GetHandle())
	if s.stack[ptr] {
		return "", fmt.Errorf("Converting circular structure to JSON")
	}
	s.stack[ptr] = true
	defer delete(s.stack, ptr)

	nextIndent := indent + s.space
	switch obj := obj.(type) {
	case *native.ObjArr:
		if obj.Length() == 0 {
			return "[]", nil
		}
		items := make([]string, obj.Length())
		for i := 0; i < obj.Length(); i++ {
			childHolder := native.NewObjectHash()
			childHolder.SetMember(jsonKey("0"), arrayAt(obj, i))
			item, err := s.serialize("0", value.EncodeHandle(heap.Allocate(childHolder)), nextIndent)
			if err != nil {
				return "", err
			}
			if item == "" {
				item = "null"
			}
			items[i] = item
		}
		if s.space == "" {
			return "[" + strings.Join(items, ",") + "]", nil
		}
		return "[\n" + nextIndent + strings.Join(items, ",\n"+nextIndent) + "\n" + indent + "]", nil
	case *native.ObjObject:
		keys := []string{}
		if len(s.propertyList) > 0 {
			keys = s.propertyList
		} else {
			for _, item := range orderedEntriesForVM(obj) {
				if len(item.Key) > 0 && item.Key[0] == 0 {
					continue
				}
				if item.Entry.Enumerable() {
					keys = append(keys, item.Key)
				}
			}
		}
		parts := []string{}
		for _, key := range keys {
			childHolder := native.NewObjectHash()
			childHolder.SetMember(jsonKey(key), obj.GetMember(jsonKey(key)))
			item, err := s.serialize(key, value.EncodeHandle(heap.Allocate(childHolder)), nextIndent)
			if err != nil {
				return "", err
			}
			if item == "" {
				continue
			}
			if s.space == "" {
				parts = append(parts, s.quote(key)+":"+item)
			} else {
				parts = append(parts, s.quote(key)+": "+item)
			}
		}
		if len(parts) == 0 {
			return "{}", nil
		}
		if s.space == "" {
			return "{" + strings.Join(parts, ",") + "}", nil
		}
		return "{\n" + nextIndent + strings.Join(parts, ",\n"+nextIndent) + "\n" + indent + "}", nil
	}
	return "", nil
}

func (vm *VM) callJSONStringify(argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	val := value.UNDEFINED
	if len(args) > 0 {
		val = args[0]
	}
	s := &jsonStringifier{vm: vm, stack: map[uint32]bool{}}
	if len(args) > 1 && args[1] != value.UNDEFINED && args[1] != value.NULL {
		if args[1].IsObject() {
			if obj, _ := heap.GetObject(args[1].GetHandle()); obj != nil {
				if _, ok := obj.(object.Callable); ok {
					s.replacer = args[1]
				} else if arr, ok := obj.(*native.ObjArr); ok {
					seen := map[string]bool{}
					for i := 0; i < arr.Length(); i++ {
						item := arrayAt(arr, i)
						if str, ok := vm.valueAsString(item); ok && !seen[str] {
							seen[str] = true
							s.propertyList = append(s.propertyList, str)
						}
					}
				}
			}
		}
	}
	if len(args) > 2 && args[2] != value.UNDEFINED {
		space := args[2]
		if space.IsNumber() {
			n := int(space.AsNumber())
			if n > 10 {
				n = 10
			}
			if n > 0 {
				s.space = strings.Repeat(" ", n)
			}
		} else if str, ok := vm.valueAsString(space); ok {
			units := native.UTF16Units(str)
			if len(units) > 10 {
				units = units[:10]
			}
			s.space = native.UTF16FromUnits(units)
		}
	}
	holder := native.NewObjectHash()
	holder.SetMember(jsonKey(""), val)
	text, err := s.serialize("", value.EncodeHandle(heap.Allocate(holder)), "")
	if err != nil {
		return value.UNDEFINED, err
	}
	if text == "" {
		return value.UNDEFINED, nil
	}
	return value.EncodeHandle(heap.Allocate(native.NewLightString(text))), nil
}
