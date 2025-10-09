package vm

import (
	"fmt"
	"go_js/allocator"
	"go_js/chunk"
	"go_js/compiler"
	"go_js/eventloop"
	"go_js/native"
	"go_js/object"
	"go_js/queue"
	"go_js/stringer"
	"go_js/value"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const STACK_MAX = math.MaxUint8
const FRAMES_MAX = 64

var ROOT_SCRIPT_LOCATION string
var globals []value.Value
var imports = make(map[string]value.Value)
var heapVars = make(map[int][]value.Value)
var heapScopesCount int

type CallFrame struct {
	fn         object.Callable
	thisCtx    value.Value
	localStart int
	returnIp   int
}

func InitFileRoot(path string) {
	ROOT_SCRIPT_LOCATION = path
}

func (cf *CallFrame) initCallFrame(fn object.Callable, localStart int, returnIp int, this value.Value) {
	cf.fn = fn
	cf.localStart = localStart
	cf.returnIp = returnIp
	cf.thisCtx = this
}

type ExceptionState struct {
	stackTop int
	jumpTo   int
	frame    int
}

type VM struct {
	frames         []CallFrame
	frameCount     int
	stack          []value.Value
	stackTop       int
	exceptionStack []ExceptionState

	exports *native.ObjObject

	debug bool
}

func NewVM(debug bool) *VM {
	frames := make([]CallFrame, FRAMES_MAX)
	stack := make([]value.Value, STACK_MAX)

	return &VM{
		frames:         frames,
		frameCount:     0,
		stack:          stack,
		stackTop:       0,
		exceptionStack: []ExceptionState{},
		debug:          debug,
	}
}

func (vm *VM) currentFrame() CallFrame {
	return vm.frames[vm.frameCount-1]
}

func (vm *VM) push(v value.Value) {
	vm.stack[vm.stackTop] = v
	vm.stackTop++
}

func (vm *VM) peek() value.Value {
	return vm.stack[vm.stackTop-1]
}

func (vm *VM) pop() value.Value {
	vm.stackTop--
	return vm.stack[vm.stackTop]
}

func (vm *VM) addGlobal(v value.Value) {
	globals = append(globals, v)
}

func (vm *VM) getGlobal(global int) value.Value {
	return globals[global]
}

func (vm *VM) concatenate(a, b value.Value) value.Value {
	if !a.IsObject() && b.IsObject() {
		a = value.EncodeHandle(allocator.Allocate(native.LightString(stringer.String(a))))
	}

	if a.IsObject() {
		var bObj object.Object
		if !b.IsObject() {
			bObj = native.LightString(stringer.String(b))
		}
		aObj, _ := allocator.GetObject(a.GetHandle())

		if bObj == nil {
			bObj, _ = allocator.GetObject(b.GetHandle())
		}

		switch aObj := aObj.(type) {
		case *native.ObjStringBuilder:
			{
				switch str := bObj.(type) {
				case *native.ObjStringBuilder:
					{
						aObj.Concatenate(string(str.Flush()))
					}
				default:
					{
						aObj.Concatenate(bObj.String())
					}
				}
				return a
			}
		case native.LightString:
			{
				builder := native.NewStringBuilder(aObj)
				switch str := bObj.(type) {
				case *native.ObjStringBuilder:
					{
						builder.Concatenate(string(str.Flush()))
					}
				default:
					{
						builder.Concatenate(bObj.String())
					}
				}

				return value.EncodeHandle(allocator.Allocate(builder))
			}
		case *native.ObjString:
			{
				switch str := bObj.(type) {
				case *native.ObjStringBuilder:
					{
						return value.EncodeHandle(allocator.Allocate(native.LightString(aObj.String() + string(str.Flush()))))
					}
				default:
					{
						return value.EncodeHandle(allocator.Allocate(native.LightString(aObj.String() + bObj.String())))
					}
				}
			}
		}
	}

	return value.Value(math.Float64bits(a.AsNumber() + b.AsNumber()))
}

func (vm *VM) subtract(a, b value.Value) value.Value {
	aIsObject, _ := object.IsValueObject(a)
	bIsObject, _ := object.IsValueObject(b)
	if aIsObject || bIsObject {
		if aIsObject && bIsObject {
			// todo
		} else {
			return value.STANDARD_NAN
		}
	}

	return value.ValueFromFloat64(a.AsNumber() - b.AsNumber())
}

// ORDER IS IMPORTANT! thanks.
func compBoolToNum(boolean value.Value, number value.Value) bool {
	if !boolean.IsBoolean() {
		panic("you didnt listen")
	}
	cmpTo := 0.0

	if truthyValue(boolean) {
		cmpTo = 1.0
	}

	return number.AsNumber() == cmpTo
}

func compNumToObject(num value.Value, obj object.Object) bool {
	numString := strconv.FormatFloat(num.AsNumber(), 'f', -1, 64)
	switch obj := obj.(type) {
	case native.LightString:
		return numString == obj.String()
	case *native.ObjString:
		return numString == obj.String()
	case *native.ObjStringBuilder:
		return numString == obj.Flush().String()
	default:
		return numString == stringer.ObjToPrimitive(obj)
	}
}

func compBoolToObject(boolean value.Value, obj object.Object) bool {
	cmpTo := 0.0

	if truthyValue(boolean) {
		cmpTo = 1.0
	}

	str := ""

	switch obj := obj.(type) {
	case native.LightString:
		str = obj.String()
	case *native.ObjString:
		str = obj.String()
	case *native.ObjStringBuilder:
		str = obj.Flush().String()
	default:
		str = stringer.ObjToPrimitive(obj)
	}

	if len(str) == 0 {
		str = "0"
	}

	res, err := strconv.ParseFloat(str, 64)

	if err != nil {
		fmt.Println(err)
		return false
	}

	return res == cmpTo
}

func compStringToObject(str object.Object, obj object.Object) bool {
	if obj.Type() == object.OBJ_STRING_BUILDER {
		return str.String() == obj.(*native.ObjStringBuilder).Flush().String()
	}

	return str.String() == stringer.ObjToPrimitive(obj)
}

func looseComparison(a, b value.Value) bool {
	if a.IsBoolean() && b.IsBoolean() {
		return a == b
	}

	if a.IsNumber() && b.IsBoolean() {
		return compBoolToNum(b, a)
	}

	if a.IsBoolean() && b.IsNumber() {
		return compBoolToNum(a, b)
	}

	if a.IsObject() && b.IsObject() {
		aObj, _ := allocator.GetObject(a.GetHandle())
		bObj, _ := allocator.GetObject(b.GetHandle())

		if (aObj.Type() == object.OBJ_STRING) && bObj.Type() == object.OBJ_STRING {
			return aObj.String() == bObj.String()
		}

		if aObj.Type() == object.OBJ_STRING && bObj.Type() != object.OBJ_STRING {
			return compStringToObject(aObj, bObj)
		}

		if aObj.Type() != object.OBJ_STRING && bObj.Type() == object.OBJ_STRING {
			return compStringToObject(bObj, aObj)
		}

		return allocator.GetPointer(a.GetHandle()) == allocator.GetPointer(b.GetHandle())
	}

	if (a == value.NULL || a == value.UNDEFINED) && (b == value.NULL || b == value.UNDEFINED) {
		return true
	}

	if a.IsObject() == !b.IsObject() {
		obj, _ := allocator.GetObject(a.GetHandle())
		if b.IsNumber() {
			return compNumToObject(b, obj)
		}

		if b.IsBoolean() {
			return compBoolToObject(b, obj)
		}
	}

	if !a.IsObject() == b.IsObject() {
		bObj, _ := allocator.GetObject(b.GetHandle())

		if a.IsNumber() {
			return compNumToObject(a, bObj)
		}

		if a.IsBoolean() {
			return compBoolToObject(a, bObj)
		}
	}

	return truthyValue(a) == truthyValue(b)
}

func truthyValue(v value.Value) bool {
	if v.IsObject() {
		obj, _ := allocator.GetObject(v.GetHandle())

		switch obj := obj.(type) {
		case *native.ObjString:
			return len(obj.Value) > 0
		case native.LightString:
			return len(string(obj)) > 0
		case *native.StringConstructor:
			return len(obj.String()) > 0
		default:
			return true
		}
	}

	if v.IsType(value.NULL) || v.IsType(value.UNDEFINED) {
		return false
	}

	if v.IsNumber() {
		return v.AsNumber() > 0
	}

	return value.TRUE == v
}

func (vm *VM) popN(n int) []value.Value {
	r := make([]value.Value, n)
	copy(r, vm.stack[vm.stackTop-n:vm.stackTop])
	vm.stackTop -= n
	return r
}

func (vm *VM) CreateTemplateString(o *object.ObjTemplateLiteral) value.Value {
	objStr := native.NewObjString(o.CreateString())
	return value.EncodeHandle(allocator.Allocate(objStr))
}

func (vm *VM) peekN(i int) value.Value {
	return vm.stack[vm.stackTop-(i+1)]
}

func (vm *VM) findArgStart() []value.Value {
	arguments := []value.Value{}

	v := vm.pop()
	for v != value.TAG_ARGUMENT_START {
		arguments = append(arguments, v)
		v = vm.pop()
	}
	slices.Reverse(arguments)

	return arguments
}

func (vm *VM) Call(fn object.Callable, currentIp *int, this value.Value, argCount int) (f CallFrame, c value.ValueChunk) {
	localStart := max(vm.stackTop-argCount, 0)
	returnIp := 0

	if currentIp != nil {
		returnIp = *currentIp
	}

	vm.frames[vm.frameCount].initCallFrame(fn, localStart, returnIp, this)
	vm.frameCount++

	if currentIp != nil {
		*currentIp = 0
	}

	f = vm.currentFrame()
	c = *f.fn.ValueChunk()

	if fn.HasArguments() {
		items := vm.popN(argCount)
		arr := native.NewArrayFrom(items)
		v := value.EncodeHandle(allocator.Allocate(arr))
		vm.push(v)
	}

	if fn.HasRestParameter() {
		arguments := vm.findArgStart()
		f.localStart = vm.stackTop

		arr := native.NewArrayFrom(arguments[fn.GetArity()-1:])
		for _, v := range arguments[:fn.GetArity()-1] {
			vm.push(v)
		}
		v := value.EncodeHandle(allocator.Allocate(arr))
		vm.push(v)

	}

	return f, c
}

func (vm *VM) Run(wg *sync.WaitGroup) {
	wg.Add(1)
Run:
	callback, errDequeue := queue.Dequeue()
	for errDequeue == nil {
		if len(callback.Stack) > 0 {
			for _, v := range callback.Stack {
				vm.push(v)
			}
		}

		f, c := vm.Call(callback.Fn, nil, callback.ThisCtx, callback.Fn.GetArity())
		_, err := vm.run(f, c)

		if err != nil {
			log.Fatalf("runtime error: %s", err.Error())
		}
		callback, errDequeue = queue.Dequeue()
	}

	if vm.debug {
		fmt.Println()
		fmt.Println("-- QUEUE DRAINED --")
	}

	tick := time.NewTicker(100 * time.Millisecond)
	hasWork := eventloop.HasWork()

	if vm.debug {
		fmt.Printf("eventloop has work: %v\n", hasWork)
	}
	for hasWork {
		select {
		case <-queue.QueueC:
			{
				goto Run
			}
		case <-tick.C:
			{
				hasWork = eventloop.HasWork()
				continue
			}
		}
	}
	wg.Done()
}

func (vm *VM) run(f CallFrame, c value.ValueChunk) (value.Value, error) {
	ip := 0

	if promise, ok := f.fn.(*native.ObjAsyncFunction); ok {

		if promise.State != nil {
			if vm.debug {
				fmt.Println()
				fmt.Printf("-- RETURNING PROMISE %v --\n", promise.String())
				printStack(promise.State.Stack)
				fmt.Println()
			}
			for _, v := range promise.State.Stack {
				vm.push(v)
			}

			ip = promise.State.Ip
		}
	}

	if vm.debug {
		fmt.Println()
		fmt.Println("-- NEW RUNNER SPAWNED --")
		fmt.Println()
		PrintChunk(c)
	}

	for {
		// time.Sleep(time.Millisecond * 100)
		code := c.Code[ip]
		ip++

		if vm.debug {
			fmt.Println(f.fn.String())
			printStack(vm.stack[0:vm.stackTop])
			fmt.Println(opNames[code])
		}

		switch code {
		case chunk.OP_CONSTANT:
			{
				vm.push(c.Constants[c.Code[ip]])
				ip++
			}
		case chunk.OP_POP:
			{
				vm.pop()
			}
		case chunk.OP_PUSH_CURRENT:
			{
				vm.push(vm.peek())
			}
		case chunk.OP_ADD:
			{
				b := vm.pop()
				a := vm.pop()

				vm.push(vm.concatenate(a, b))
			}
		case chunk.OP_SUBTRACT:
			{
				b := vm.pop()
				a := vm.pop()

				vm.push(vm.subtract(a, b))
			}
		case chunk.OP_DIVIDE:
			{
				b := vm.pop()
				a := vm.pop()
				// type checks required
				vm.push(value.ValueFromFloat64(a.AsNumber() / b.AsNumber()))
			}
		case chunk.OP_MULTIPLY:
			{
				b := vm.pop()
				a := vm.pop()
				// type checks required
				vm.push(value.ValueFromFloat64(a.AsNumber() * b.AsNumber()))
			}
		case chunk.OP_EXPONENTIATION:
			{

				b := vm.pop()
				a := vm.pop()
				// type checks required
				vm.push(value.ValueFromFloat64(math.Pow(a.AsNumber(), b.AsNumber())))
			}
		case chunk.OP_MODULO:
			{
				b := vm.pop()
				a := vm.pop()
				// type checks required
				vm.push(value.ValueFromFloat64(math.Mod(a.AsNumber(), b.AsNumber())))
			}
		case chunk.OP_LESS_THAN:
			{
				b := vm.pop()
				a := vm.pop()

				if a.AsNumber() < b.AsNumber() {
					vm.push(value.TRUE)
				} else {
					vm.push(value.FALSE)
				}
			}
		case chunk.OP_LESS_THAN_EQUAL:
			{
				b := vm.pop()
				a := vm.pop()

				if a.AsNumber() <= b.AsNumber() {
					vm.push(value.TRUE)
				} else {
					vm.push(value.FALSE)
				}
			}
		case chunk.OP_GREATER_THAN:
			{
				b := vm.pop()
				a := vm.pop()

				if a.AsNumber() > b.AsNumber() {
					vm.push(value.TRUE)
				} else {
					vm.push(value.FALSE)
				}
			}
		case chunk.OP_GREATER_THAN_EQUAL:
			{
				b := vm.pop()
				a := vm.pop()

				if a.AsNumber() >= b.AsNumber() {
					vm.push(value.TRUE)
				} else {
					vm.push(value.FALSE)
				}
			}
		case chunk.OP_EQUALS:
			{
				b := vm.pop()
				a := vm.pop()

				if looseComparison(a, b) {
					vm.push(value.TRUE)
				} else {
					vm.push(value.FALSE)
				}
			}
		case chunk.OP_STRICT_EQUALS:
			{
				b := vm.pop()
				a := vm.pop()

				if b.IsObject() && a.IsObject() {
					if allocator.GetPointer(b.GetHandle()) == allocator.GetPointer(a.GetHandle()) {
						vm.push(value.TRUE)
					} else {
						vm.push(value.FALSE)
					}
				} else if a == b {
					vm.push(value.TRUE)
				} else {
					vm.push(value.FALSE)
				}
			}
		case chunk.OP_STRICT_NOT_EQUALS:
			{
				b := vm.pop()
				a := vm.pop()

				if b.IsObject() && a.IsObject() {
					if allocator.GetPointer(b.GetHandle()) != allocator.GetPointer(a.GetHandle()) {
						vm.push(value.TRUE)
					} else {
						vm.push(value.FALSE)
					}
				} else if a != b {
					vm.push(value.TRUE)
				} else {
					vm.push(value.FALSE)
				}
			}
		case chunk.OP_LOGICAL_OR:
			{
				right := vm.pop()
				left := vm.pop()

				if right.IsBoolean() && left.IsBoolean() {
					if truthyValue(left) {
						vm.push(value.TRUE)
					} else {
						vm.push(right)
					}
				}

				if truthyValue(left) {
					vm.push(left)
				} else {
					if truthyValue(right) {
						vm.push(right)
					} else {
						vm.push(value.FALSE)
					}
				}
			}
		case chunk.OP_LOGICAL_AND:
			{
				right := vm.pop()
				left := vm.pop()

				if right.IsBoolean() && left.IsBoolean() {
					if truthyValue(left) && truthyValue(right) {
						vm.push(value.TRUE)
					} else {
						vm.push(value.FALSE)
					}
					continue
				}

				if truthyValue(left) {
					if truthyValue(right) {
						vm.push(right)
					}
				} else {
					vm.push(left)
				}
			}
		case chunk.OP_IN:
			{
				obj := vm.pop()
				prop := vm.pop()

				has := value.FALSE

				if obj, err := allocator.GetObject(obj.GetHandle()); err == nil {
					v := obj.(object.Hashable).GetMember(prop)

					if v != value.UNDEFINED {
						has = value.TRUE
					}
				}

				vm.push(has)
			}
		case chunk.OP_NULL_COALESHING:
			{
				right := vm.pop()
				left := vm.pop()

				if left == value.NULL || left == value.UNDEFINED {
					vm.push(right)
				} else {
					vm.push(left)
				}
			}
		case chunk.OP_NEGATE:
			{
				v := vm.pop()

				if v.IsNumber() {
					vm.push(value.ValueFromFloat64(-(v.AsNumber())))
				} else {
					return value.UNDEFINED, fmt.Errorf("no support for negating %s yet", stringer.String(v))
				}
			}
		case chunk.OP_ARG_START:
			{
				vm.push(value.TAG_ARGUMENT_START)
			}
		case chunk.OP_SPREAD:
			{
				v := vm.pop()

				if !v.IsObject() {
					return value.UNDEFINED, fmt.Errorf("%v cannot be spread", stringer.String(v))
				}
				obj, _ := allocator.GetObject(v.GetHandle())

				switch obj := obj.(type) {
				case *native.ObjArr:
					for _, v := range obj.Values() {
						vm.push(v)
					}
				default:
					return value.UNDEFINED, fmt.Errorf("unsupported spread operation for object %v", stringer.String(v))
				}
			}
		case chunk.OP_JUMP_IF_FALSE:
			{
				v := vm.pop()
				jump := c.ReadInt(&ip)

				if !truthyValue(v) {
					ip = jump
				}
			}
		case chunk.OP_JUMP_IF_TRUE:
			{
				v := vm.pop()
				jump := c.ReadInt(&ip)

				if truthyValue(v) {
					ip = jump
				}
			}
		case chunk.OP_JUMP:
			{
				jump := c.ReadInt(&ip)
				ip = jump
			}
		case chunk.OP_DEFINE_HEAP_VAR:
			{
				variable := vm.pop()
				if variable == value.TAG_METHOD_HANDLE {
					variable = vm.pop()
					vm.pop() // pop this context
				}
				if scope, found := heapVars[f.fn.GetHeapScope()]; found {
					scope = append(scope, variable)
					heapVars[f.fn.GetHeapScope()] = scope
				} else {
					panic("no heap scope generated for function")
				}
			}
		case chunk.OP_GET_HEAP_VAR:
			{
				heapVar := c.Code[ip]
				ip++
				v := heapVars[f.fn.GetHeapScope()][heapVar]
				if v.IsObject() {
					obj, err := allocator.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to fetch object in OP_GET_GLOBAL %e", err)
					}

					if _, ok := obj.(object.NeedsContext); ok {
						vm.push(vm.currentFrame().thisCtx)
						vm.push(v)
						vm.push(value.TAG_METHOD_HANDLE)
						continue
					}
				}
				vm.push(v)
			}
		case chunk.OP_SET_HEAP_VAR:
			{
				heapVar := c.Code[ip]
				ip++
				heapVars[f.fn.GetHeapScope()][heapVar] = vm.pop()
			}
		case chunk.OP_DEFINE_GLOBAL:
			{
				v := vm.pop()
				if v == value.TAG_METHOD_HANDLE {
					v = vm.pop()
					vm.pop() // pop this context
				}
				if v.IsObject() {
					obj, err := allocator.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to receive object at OP_DEFINE_GLOBAL %s", err)
					}
					switch obj := obj.(type) {
					case *object.ObjFunction:
						{
							if obj.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
								v = value.EncodeHandle(allocator.Allocate(obj.Clone()))
							}
						}
					case *native.ObjStringBuilder:
						{
							v = value.EncodeHandle(allocator.Allocate(obj.Flush()))
						}
					}
				}
				vm.addGlobal(v)
			}
		case chunk.OP_GET_GLOBAL:
			{
				global := c.Code[ip]
				ip++
				v := vm.getGlobal(int(global))

				if v.IsObject() {
					obj, err := allocator.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to fetch object in OP_GET_GLOBAL %e", err)
					}

					if _, ok := obj.(object.NeedsContext); ok {
						vm.push(vm.currentFrame().thisCtx)
						vm.push(v)
						vm.push(value.TAG_METHOD_HANDLE)
						continue
					}
				}
				vm.push(v)
			}
		case chunk.OP_SET_GLOBAL:
			{
				global := c.Code[ip]
				ip++
				v := vm.pop()
				if v.IsObject() {
					obj, err := allocator.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to receive object at OP_DEFINE_GLOBAL %s", err)
					}
					switch obj := obj.(type) {
					case *object.ObjFunction:
						{
							if obj.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
								v = value.EncodeHandle(allocator.Allocate(obj.Clone()))
							}
						}
					}
				}
				globals[global] = v
			}
		case chunk.OP_POP_LOCAL:
			{
				vm.stackTop--
			}
		case chunk.OP_DEFINE_LOCAL:
			{
				v := vm.pop()
				if v == value.TAG_METHOD_HANDLE {
					v = vm.pop()
					vm.pop() // pop this context
				}

				if v.IsObject() {
					obj, err := allocator.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to receive object at OP_DEFINE_LOCAL %s", err)
					}
					switch obj := obj.(type) {
					case *object.ObjFunction:
						{
							if obj.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
								vm.push(value.EncodeHandle(allocator.Allocate(obj.Clone())))
								vm.pop()
							}
						}
					case *native.ObjPromise:
						{
							vm.push(obj.Value)
							vm.pop()
						}
					}
				}
				vm.stackTop++
			}
		case chunk.OP_GET_LOCAL:
			{
				slot := int(c.Code[ip])
				ip++
				v := vm.stack[f.localStart+slot]

				if v.IsObject() {
					obj, err := allocator.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to fetch object in OP_GET_LOCAL %e", err)
					}

					if _, ok := obj.(object.NeedsContext); ok {
						vm.push(vm.currentFrame().thisCtx)
						vm.push(v)
						vm.push(value.TAG_METHOD_HANDLE)
						continue
					}
				}
				vm.push(v)
			}
		case chunk.OP_SET_LOCAL:
			{
				slot := int(c.Code[ip])
				ip++
				v := vm.pop()
				if v.IsObject() {
					obj, err := allocator.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to receive object at OP_DEFINE_GLOBAL %s", err)
					}
					switch obj := obj.(type) {
					case *object.ObjFunction:
						{
							if obj.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
								v = value.EncodeHandle(allocator.Allocate(obj.Clone()))
							}
						}
					case *native.ObjStringBuilder:
						{
							v = value.EncodeHandle(allocator.Allocate(obj.Flush()))
						}
					}
				}
				vm.stack[f.localStart+slot] = v
			}
		case chunk.OP_CREATE_OBJECT:
			{
				objHash := native.NewObjectHash()
				handle := allocator.Allocate(objHash)

				vm.push(value.EncodeHandle(handle))
			}
		case chunk.OP_SET_OBJECT_MEMBER:
			{
				v := vm.pop()
				k := vm.pop()

				hash := vm.peek()

				isObject, handle := object.IsValueObject(hash)

				if !isObject {
					return value.UNDEFINED, fmt.Errorf("%v is not an object", hash)
				}

				if v.IsObject() {
					obj, _ := allocator.GetObject(v.GetHandle())

					// check if we have a closure in hand
					if fn, ok := obj.(*object.ObjFunction); ok && fn.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
						v = value.EncodeHandle(allocator.Allocate(fn.Clone()))
					}
				}

				obj, _ := allocator.GetObject(handle)

				if k.IsObject() {
					keyObj, _ := allocator.GetObject(k.GetHandle())

					if b, ok := keyObj.(*native.ObjStringBuilder); ok {
						key := b.Flush()
						k = value.EncodeHandle(allocator.Allocate(key))
					}

				}

				if obj, ok := obj.(object.Hashable); ok {
					obj.SetMember(k, v)
				} else {
					return value.UNDEFINED, fmt.Errorf("we currently don't support adding properties to %s types", stringer.String(hash))
				}
			}
		case chunk.OP_GET_OBJECT_MEMBER:
			{
				member := vm.pop()
				objValue := vm.pop()

				if objValue == value.TAG_METHOD_HANDLE {
					objValue = vm.pop()
					vm.pop() // pop this context
				}

				if objValue == value.UNDEFINED {
					vm.push(value.UNDEFINED)
					continue
				}

				obj, err := allocator.GetObject(objValue.GetHandle())

				if err != nil {
					return value.UNDEFINED, fmt.Errorf("failed to get object from %s in OP_GET_OBJECT_MEMBER %s", stringer.String(objValue), err)
				}

				switch obj := obj.(type) {
				case *native.ObjArr:
					{
						if member.IsInteger() {
							value := obj.GetElementAt(int(member.AsNumber()))
							vm.push(value)
						} else {
							v := obj.GetMember(member)

							if v.IsObject() {
								member, err := allocator.GetObject(v.GetHandle())

								if err != nil {
									return value.UNDEFINED, fmt.Errorf("failed to get object in OP_GET_OBJECT_MEMBER %e", err)
								}

								if _, ok := member.(object.NeedsContext); ok {
									vm.push(objValue)
									vm.push(v)
									vm.push(value.TAG_METHOD_HANDLE)
									continue
								}
							}
							vm.push(v)
						}
					}
				case native.LightString:
					{
						boxed := native.NewObjString(obj.String())
						v := boxed.GetMember(member)

						if v.IsObject() {
							member, err := allocator.GetObject(v.GetHandle())

							if err != nil {
								return value.UNDEFINED, fmt.Errorf("failed to get object in OP_GET_OBJECT_MEMBER %e", err)
							}
							if _, ok := member.(object.NeedsContext); ok {
								vm.push(objValue)
								vm.push(value.EncodeHandle(allocator.Allocate(member)))
								vm.push(value.TAG_METHOD_HANDLE)
								continue
							}
						}
						vm.push(v)
					}
				case *native.ObjStringBuilder:
					{
						boxed := native.NewObjString(string(obj.Flush()))
						v := boxed.GetMember(member)

						if v.IsObject() {
							member, err := allocator.GetObject(v.GetHandle())

							if err != nil {
								return value.UNDEFINED, fmt.Errorf("failed to get object in OP_GET_OBJECT_MEMBER %e", err)
							}

							if _, ok := member.(object.NeedsContext); ok {
								vm.push(objValue)
								vm.push(v)
								vm.push(value.TAG_METHOD_HANDLE)
								continue
							}
						}
						vm.push(v)
					}
				case object.Hashable:
					{
						v := obj.GetMember(member)

						if v.IsObject() {
							member, err := allocator.GetObject(v.GetHandle())

							if err != nil {
								return value.UNDEFINED, fmt.Errorf("failed to get object in OP_GET_OBJECT_MEMBER %e", err)
							}
							if _, ok := member.(object.NeedsContext); ok {
								vm.push(objValue)
								vm.push(v)
								vm.push(value.TAG_METHOD_HANDLE)
								continue
							}
						}
						vm.push(v)
					}
				default:
					return value.UNDEFINED, fmt.Errorf("can't get %s from %s", stringer.String(member), stringer.String(objValue))
				}
			}
		case chunk.OP_PUSH_UNDEFINED:
			{
				vm.push(value.UNDEFINED)
			}
		case chunk.OP_CALL:
			{
				argCount := int(c.Code[ip])
				ip++
				handle := vm.pop()

				if handle != value.TAG_METHOD_HANDLE {
					panic("should always have a handle")
				}

				callee := vm.pop()
				thisCtx := vm.pop()

				fn, err := allocator.GetObject(callee.GetHandle())

				if err != nil {
					return value.UNDEFINED, err
				}

				switch fn := fn.(type) {
				case *native.ObjAsyncFunction:
					{
						fn = fn.Clone().(*native.ObjAsyncFunction)
						if !fn.ReturnArgumentIsPromise {
							promise := native.NewPromise()
							fn.SetPromise(promise)
							handle := allocator.Allocate(promise)
							vm.push(value.EncodeHandle(handle))
						}

						f, c = vm.Call(fn.Clone(), &ip, value.UNDEFINED, argCount)
						f.localStart -= int(argCount)
					}
				case *native.ObjGenerator:
					{
						gen := fn.Clone()
						vm.push(value.EncodeHandle(allocator.Allocate(gen)))
					}
				case object.Callable:
					{
						f, c = vm.Call(fn, &ip, thisCtx, argCount)
					}
				case *native.Resolve:
					{
						v := vm.pop()
						fn := native.NewAsyncFunction("RESOLVED_FN", 0, nil)
						fn.ValueChunk().EmitBytes(chunk.OP_RETURN)
						promise := native.NewPromise()
						fn.SetPromise(promise)
						vm.push(value.EncodeHandle(allocator.Allocate(promise)))

						f, c = vm.Call(fn, &ip, value.UNDEFINED, 0)
						vm.push(v)
					}
				case *native.Then:
					{
						p, _ := allocator.GetObject(thisCtx.GetHandle())
						v := p.(*native.ObjPromise).Value
						callbackFn := vm.pop()

						callback, _ := allocator.GetObject(callbackFn.GetHandle())
						fn := callback.(*object.ObjFunction)

						queue.Enqueue(object.Callback{Fn: fn, ThisCtx: value.UNDEFINED, Stack: []value.Value{v}}, queue.MICRO_TASK, false)
						vm.push(value.UNDEFINED)
					}
				case *native.MapGet:
					{
						arg := vm.pop()
						owner, _ := allocator.GetObject(thisCtx.GetHandle())

						vm.push(fn.Get(owner.(*native.Map), arg))
					}
				case *native.MapHas:
					{
						arg := vm.pop()
						owner, _ := allocator.GetObject(thisCtx.GetHandle())

						vm.push(fn.Has(owner.(*native.Map), arg))
					}
				case *native.MapSet:
					{
						v := vm.pop()
						k := vm.pop()
						owner, _ := allocator.GetObject(thisCtx.GetHandle())

						fn.Set(owner.(*native.Map), k, v)
						vm.push(value.UNDEFINED)
					}
				case *native.ArrayPush:
					{
						arg := vm.pop()
						this, _ := allocator.GetObject(thisCtx.GetHandle())
						vm.push(fn.Push(this.(*native.ObjArr), arg))
					}
				case *native.ArrayForEach:
					{
						callback := vm.pop()
						this, _ := allocator.GetObject(thisCtx.GetHandle())
						iterator := object.NewValueIterator(this.(object.Iterable))
						done := iterator.Next()

						obj, err := allocator.GetObject(callback.GetHandle())

						if err != nil {
							return value.UNDEFINED, err
						}
						runner := NewVM(vm.debug)
						if fn, ok := obj.(*object.ObjFunction); ok {
							for !done {

								item := iterator.Current()
								runner.push(item)
								f, c := runner.Call(fn, nil, value.UNDEFINED, argCount)
								runner.run(f, c)
								done = iterator.Next()
							}
						} else {
							return value.UNDEFINED, fmt.Errorf("callback was not a function %s", stringer.String(callback))
						}

						vm.push(value.UNDEFINED)
					}
				case *native.ArrayFilter:
					{
						callback := vm.pop()
						this, _ := allocator.GetObject(thisCtx.GetHandle())
						iterator := object.NewValueIterator(this.(object.Iterable))
						done := iterator.Next()

						obj, err := allocator.GetObject(callback.GetHandle())

						if err != nil {
							return value.UNDEFINED, err
						}

						arr := []value.Value{}
						runner := NewVM(vm.debug)
						if fn, ok := obj.(*object.ObjFunction); ok {
							for !done {

								item := iterator.Current()
								runner.push(item)
								f, c := runner.Call(fn, nil, value.UNDEFINED, argCount)
								result, err := runner.run(f, c)

								if err != nil {
									return value.UNDEFINED, err
								}

								if truthyValue(result) {
									arr = append(arr, item)
								}

								done = iterator.Next()
							}
						} else {
							return value.UNDEFINED, fmt.Errorf("callback was not a function %s", stringer.String(callback))
						}
						length := len(arr)
						objArr := native.NewArray(length)

						for _, item := range arr {
							objArr.PushElement(item)
						}

						v := value.EncodeHandle(allocator.Allocate(objArr))
						vm.push(v)
					}
				case *native.ArrayMap:
					{
						callback := vm.pop()
						this, _ := allocator.GetObject(thisCtx.GetHandle())
						iterator := object.NewValueIterator(this.(object.Iterable))
						done := iterator.Next()

						obj, err := allocator.GetObject(callback.GetHandle())

						if err != nil {
							return value.UNDEFINED, err
						}

						arr := []value.Value{}
						runner := NewVM(vm.debug)
						if fn, ok := obj.(*object.ObjFunction); ok {
							for !done {

								item := iterator.Current()
								runner.push(item)
								f, c := runner.Call(fn, nil, value.UNDEFINED, argCount)
								result, err := runner.run(f, c)

								if err != nil {
									return value.UNDEFINED, err
								}

								arr = append(arr, result)
								done = iterator.Next()
							}
						} else {
							return value.UNDEFINED, fmt.Errorf("callback was not a function %s", stringer.String(callback))
						}
						length := len(arr)
						objArr := native.NewArray(length)

						for _, item := range arr {
							objArr.PushElement(item)
						}

						v := value.EncodeHandle(allocator.Allocate(objArr))
						vm.push(v)
					}
				case *native.ArrayReduce:
					{
						initialValue := vm.pop()
						callback := vm.pop()
						this, _ := allocator.GetObject(thisCtx.GetHandle())
						iterator := object.NewValueIterator(this.(object.Iterable))
						done := iterator.Next()

						obj, err := allocator.GetObject(callback.GetHandle())

						if err != nil {
							return value.UNDEFINED, err
						}

						runner := NewVM(vm.debug)
						if fn, ok := obj.(*object.ObjFunction); ok {
							for !done {
								item := iterator.Current()
								runner.push(initialValue)
								runner.push(item)
								f, c := runner.Call(fn, nil, value.UNDEFINED, argCount)
								result, err := runner.run(f, c)

								if err != nil {
									return value.UNDEFINED, err
								}

								initialValue = result
								done = iterator.Next()
							}
						} else {
							return value.UNDEFINED, fmt.Errorf("callback was not a function %s", stringer.String(callback))
						}
						vm.push(initialValue)
					}
				case *native.StringToUpperCase:
					{
						this, _ := allocator.GetObject(thisCtx.GetHandle())
						vm.push(value.EncodeHandle(allocator.Allocate(fn.ToUpperCase(this.String()))))
					}
				case *native.StringIncludes:
					{
						arg := vm.pop()
						this, _ := allocator.GetObject(thisCtx.GetHandle())
						vm.push(fn.Includes(this.String(), stringer.String(arg)))
					}
				case *native.ToString:
					{
						this, _ := allocator.GetObject(thisCtx.GetHandle())
						handle := allocator.Allocate(native.NewObjString(fn.ToString(this)))
						vm.push(value.EncodeHandle(handle))
					}
				case *native.Log:
					{
						arg := vm.pop()

						if arg.IsObject() {
							obj, err := allocator.GetObject(arg.GetHandle())

							if err != nil {
								return value.UNDEFINED, fmt.Errorf("couldn't receive argument at native.Log %s", err)
							}

							if b, ok := obj.(*native.ObjStringBuilder); ok {
								arg = value.EncodeHandle(allocator.Allocate(b.Flush()))
							}
						}

						vm.log(arg)
						vm.push(value.UNDEFINED)
					}
				case *native.Next:
					{
						this, _ := allocator.GetObject(thisCtx.GetHandle())
						generator := this.(*native.ObjGenerator)
						// need to fix the -2, it's because OP_PUSH_UNDEFINED and OP_RETURN are currently added automatically to functions (if they are missing)
						if generator.Ip == len(generator.ValueChunk().Code)-2 {
							d := native.NewObjectHash()
							d.SetMember(native.KEY_DONE, value.TRUE)
							vm.push(value.EncodeHandle(allocator.Allocate(d)))
							continue
						}

						f, c = vm.Call(generator, &ip, value.UNDEFINED, argCount)

						ip = generator.Ip

						if len(generator.Locals) > 0 {
							for _, v := range generator.Locals {
								vm.push(v)
								vm.pop()
								vm.stackTop++
							}
						}
					}
				case *native.ObjectKeys:
					{
						arg := vm.pop()
						arr := fn.Keys(arg)

						length := len(arr)
						objArr := native.NewArray(length)

						for _, item := range arr {
							objArr.PushElement(item)
						}

						v := value.EncodeHandle(allocator.Allocate(objArr))
						vm.push(v)
					}
				case *native.ObjectValues:
					{
						arg := vm.pop()
						arr := fn.Values(arg)

						length := len(arr)
						objArr := native.NewArray(length)

						for _, item := range arr {
							objArr.PushElement(item)
						}

						v := value.EncodeHandle(allocator.Allocate(objArr))
						vm.push(v)
					}
				case *native.ResolveFunc:
					{
						if vm.stackTop == 0 {
							vm.push(value.UNDEFINED)
						}

						v := vm.pop()
						fn.Resolve(v)
						vm.push(value.UNDEFINED)
					}
				case *native.SetTimeout:
					{
						ms := vm.pop().AsNumber()
						callback := vm.pop()

						handle := callback.GetHandle()
						obj, err := allocator.GetObject(handle)

						if err != nil {
							return value.UNDEFINED, err
						}

						if callback, ok := obj.(*object.ObjFunction); ok {
							fn.Set(int(ms), callback, thisCtx)
							eventloop.Dispatch(fn.Clone())
							vm.push(value.UNDEFINED)
						}
					}
				case *native.Now:
					vm.push(fn.Now())
				case *native.Create:
					{
						arg := vm.pop()
						o := native.NewObjectFromObject(arg)
						vm.push(value.EncodeHandle(allocator.Allocate(o)))
					}
				case *native.HasOwnProperty:
					{
						arg := vm.pop()

						if fn.HasOwn(thisCtx, arg) {
							vm.push(value.TRUE)
						} else {
							vm.push(value.FALSE)
						}
					}
				case *native.SetAdd:
					{
						arg := vm.pop()
						owner, _ := allocator.GetObject(thisCtx.GetHandle())
						fn.Add(owner.(*native.Set), arg)
						vm.push(value.UNDEFINED)
					}
				case *native.SetHas:
					{
						arg := vm.pop()
						owner, _ := allocator.GetObject(thisCtx.GetHandle())
						vm.push(fn.Has(owner.(*native.Set), arg))
					}
				case *native.QueueMicroTask:
					{
						v := vm.pop()

						fn, _ := allocator.GetObject(v.GetHandle())
						queue.Enqueue(object.Callback{Fn: fn.(object.Callable), ThisCtx: value.UNDEFINED, Stack: []value.Value{v}}, queue.MICRO_TASK, false)
					}
				}

			}
		case chunk.OP_RETURN:
			{
				if promise, ok := f.fn.(*native.ObjAsyncFunction); ok {
					ip = f.returnIp
					value := value.UNDEFINED
					vm.frameCount--

					if vm.stackTop > 0 {
						value = vm.pop()
					}

					promise.Resolve(value)

					if vm.frameCount <= 0 {
						vm.stackTop = 0

						if vm.debug {
							fmt.Println("--")
						}
						return value, nil
					}

					vm.stackTop = f.localStart

					if promise.ReturnArgumentIsPromise {
						vm.push(value)
					}

					f = vm.currentFrame()
					c = *f.fn.ValueChunk()
				} else {
					ip = f.returnIp
					value := value.UNDEFINED
					vm.frameCount--

					if vm.stackTop > 0 {
						value = vm.pop()
					}

					if vm.frameCount <= 0 {
						vm.stackTop = 0

						if vm.debug {
							fmt.Println("--")
						}
						return value, nil
					}

					vm.stackTop = f.localStart
					vm.push(value)

					f = vm.currentFrame()
					c = *f.fn.ValueChunk()
				}
			}
		case chunk.OP_CREATE_ARRAY:
			{
				length := c.ReadInt(&ip)
				arr := native.NewArray(length)
				handle := allocator.Allocate(arr)
				vm.push(value.EncodeHandle(handle))
			}
		case chunk.OP_PUSH_ELEMENT:
			{
				v := vm.pop()
				arr := vm.peek()

				obj, err := allocator.GetObject(arr.GetHandle())

				if err != nil {
					return value.UNDEFINED, err
				}

				arrOBj, ok := obj.(*native.ObjArr)

				if !ok {
					return value.UNDEFINED, fmt.Errorf("trying to initialize {%s} that is not an array", stringer.String(arr))
				}

				arrOBj.PushElement(v)
			}
		case chunk.OP_GET_ITERATOR:
			{
				iteratee := vm.pop()
				type_ := c.Code[ip]
				ip++

				if !iteratee.IsObject() {
					return value.UNDEFINED, fmt.Errorf("%s is not an object", stringer.String(iteratee))
				}

				obj, err := allocator.GetObject(iteratee.GetHandle())

				if err != nil {
					return value.UNDEFINED, err
				}

				iteratorObj, ok := obj.(object.Iterable)

				if !ok {
					return value.UNDEFINED, fmt.Errorf("%s is not iterable", stringer.String(iteratee))
				}

				var iterator *object.Iterator

				if type_ == compiler.ITERATOR_FOR_IN {
					iterator = object.NewKeyIterator(iteratorObj)
				} else {
					iterator = object.NewValueIterator(iteratorObj)
				}

				vm.push(value.EncodeHandle(allocator.Allocate(iterator)))
			}
		case chunk.OP_ITERATOR_NEXT:
			{
				iterator := vm.peek()

				if !iterator.IsObject() {
					return value.UNDEFINED, fmt.Errorf("%s is not an object", stringer.String(iterator))
				}

				obj, err := allocator.GetObject(iterator.GetHandle())

				if err != nil {
					return value.UNDEFINED, err
				}

				iteratorObj, ok := obj.(*object.Iterator)

				if !ok {
					return value.UNDEFINED, fmt.Errorf("%s is not iterable", stringer.String(iterator))
				}

				done := iteratorObj.Next()

				if done {
					vm.push(value.TRUE)
				} else {
					vm.push(value.FALSE)
				}
			}
		case chunk.OP_ITERATOR_CURRENT:
			{
				iterator := vm.peek()

				if !iterator.IsObject() {
					return value.UNDEFINED, fmt.Errorf("%s is not an object", stringer.String(iterator))
				}

				obj, err := allocator.GetObject(iterator.GetHandle())

				if err != nil {
					return value.UNDEFINED, err
				}

				iteratorObj, ok := obj.(*object.Iterator)

				if !ok {
					return value.UNDEFINED, fmt.Errorf("%s is not iterable", stringer.String(iterator))
				}

				vm.push(iteratorObj.Current())
			}
		case chunk.OP_CREATE_HEAP_SCOPE:
			{
				heapScopesCount++
				heapVars[heapScopesCount] = []value.Value{}
				setHeapScopes(f.fn.ValueChunk(), heapScopesCount)
				f.fn.SetHeapScope(heapScopesCount)
			}
		case chunk.OP_TRY_BLOCK_START:
			{
				catchStart := int(c.Code[ip+3]) | int(c.Code[ip+2])<<8 | int(c.Code[ip+1])<<16 | int(c.Code[ip])<<24
				ip += 4
				vm.exceptionStack = append(vm.exceptionStack, ExceptionState{jumpTo: catchStart, stackTop: vm.stackTop, frame: vm.frameCount})
			}
		case chunk.OP_TRY_BLOCK_END:
			{
				vm.exceptionStack = vm.exceptionStack[:len(vm.exceptionStack)-1]
			}
		case chunk.OP_NEW:
			{
				argCount := int(c.Code[ip])
				ip++
				callee := vm.pop()
				obj, err := allocator.GetObject(callee.GetHandle())

				if err != nil {
					return value.UNDEFINED, err
				}

				switch ctor := obj.(type) {
				case *native.ObjClass:
					{
						instance := ctor.NewInstance()
						ctor := instance.GetMember(native.KEY_CTOR)

						obj, err := allocator.GetObject(ctor.GetHandle())

						if err != nil {
							return value.UNDEFINED, fmt.Errorf("contructor was not an object %s", stringer.String(ctor))
						}

						if constructor, ok := obj.(object.Callable); ok {
							builder := NewVM(vm.debug)
							fn := object.NewFunction("builder", 0, nil)
							fn.ValueChunk().EmitBytes(chunk.OP_CALL, uint8(constructor.GetArity()), chunk.OP_RETURN)
							instance := value.EncodeHandle(allocator.Allocate(instance))

							for _, v := range vm.popN(constructor.GetArity()) {
								builder.push(v)
							}

							builder.push(instance)
							builder.push(ctor)
							builder.push(value.TAG_METHOD_HANDLE)
							f, c := builder.Call(fn, nil, value.UNDEFINED, argCount)
							builder.run(f, c)

							vm.push(instance)
						} else {
							return value.UNDEFINED, fmt.Errorf("contructor was not an function %s", stringer.String(ctor))
						}

					}
				case native.Constructor:
					{
						args := vm.popN(int(argCount))
						params := make([]any, argCount)

						for i, v := range args {
							params[i] = v
						}

						newObj, err := ctor.New(params...)

						if err != nil {
							return value.UNDEFINED, err
						}

						objHandle := allocator.Allocate(newObj)
						vm.push(value.EncodeHandle(objHandle))
					}
				case *native.PromiseConstructor:
					{
						arg := vm.pop()
						handle := arg.GetHandle()
						executor, err := allocator.GetObject(handle)

						if err != nil {
							return value.UNDEFINED, err
						}

						promise := native.NewPromise()
						resolve := native.NewResolveFunc(promise)
						resolveHandle := allocator.Allocate(resolve)

						if executor, ok := executor.(object.Callable); ok {
							runner := NewVM(vm.debug)
							f, c := runner.Call(executor, nil, value.UNDEFINED, argCount)
							runner.push(value.EncodeHandle(resolveHandle))
							runner.run(f, c)
						}

						handle = allocator.Allocate(promise)
						vm.push(value.EncodeHandle(handle))
					}
				}
			}
		case chunk.OP_THROW:
			{
				err := vm.pop()
				exceptionState := vm.exceptionStack[len(vm.exceptionStack)-1]
				vm.exceptionStack = vm.exceptionStack[:len(vm.exceptionStack)-1]

				vm.frameCount = exceptionState.frame
				f = vm.currentFrame()
				c = *f.fn.ValueChunk()
				ip = exceptionState.jumpTo
				vm.stackTop = exceptionState.stackTop
				vm.push(err)
			}
		case chunk.OP_AWAIT:
			{
				awaitee := vm.pop()
				awaiteeObj, err := allocator.GetObject(awaitee.GetHandle())

				if err != nil {
					return value.FALSE, err
				}

				if promise, ok := awaiteeObj.(*native.ObjPromise); ok {
					if curentAsyncFn, ok := f.fn.(*native.ObjAsyncFunction); ok {
						count := (vm.stackTop - f.localStart) + 1 // +1 because we popped our promise
						stack := make([]value.Value, count)
						copy(stack, append(vm.stack[f.localStart:vm.stackTop], awaitee))
						curentAsyncFn.Pause(stack, ip)
						curentAsyncFn.Await(promise)

						vm.frameCount--

						if vm.frameCount > 0 {
							ip = f.returnIp
							f = vm.currentFrame()
							c = *f.fn.ValueChunk()
						}
					}
				}
			}
		case chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS:
			{
				amount := c.Code[ip]
				ip++

				removeMap := map[int]bool{}

				for range amount {
					idx := int(c.Code[ip])
					removeMap[idx] = true
					ip++
					heapVars[f.fn.GetHeapScope()] = append(heapVars[f.fn.GetHeapScope()], vm.stack[f.localStart+idx])
				}

				localCount := (vm.stackTop - f.localStart) - int(amount)

				locals := make([]value.Value, 0, localCount)

				for i, v := range vm.stack[f.localStart:vm.stackTop] {
					if !removeMap[i] {
						locals = append(locals, v)
					}
				}

				vm.stackTop = f.localStart

				for _, v := range locals {
					vm.push(v)
				}
			}
		case chunk.OP_CREATE_CLASS_START:
			{
				name := vm.pop()

				if n, err := allocator.GetObject(name.GetHandle()); err == nil && n.Type() == object.OBJ_STRING {
					name := n.String()

					class := native.NewObjClass(name)
					proto := native.NewPrototype(name)

					vm.push(value.EncodeHandle(allocator.Allocate(class)))
					vm.push(value.EncodeHandle(allocator.Allocate(proto)))
				} else {
					return value.UNDEFINED, fmt.Errorf("%s is not a string", stringer.String(name))
				}
			}
		case chunk.OP_CREATE_CLASS_END:
			{
				proto := vm.pop()
				class := vm.peek()

				classObj, _ := allocator.GetObject(class.GetHandle())
				protoObj, _ := allocator.GetObject(proto.GetHandle())

				classObj.(*native.ObjClass).SetPrototype(protoObj.(*native.Prototype))
			}
		case chunk.OP_PUSH_METHOD:
			{
				method := vm.pop()

				methodObj, err := allocator.GetObject(method.GetHandle())

				if err != nil {
					return value.UNDEFINED, fmt.Errorf("%s was not an object", stringer.String(method))
				}

				if m, ok := methodObj.(object.Callable); ok {
					if m.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
						m = m.Clone()
					}
					method = value.EncodeHandle(allocator.Allocate(m))
				} else {
					return value.UNDEFINED, fmt.Errorf("%s was not an function", stringer.String(method))
				}

				key := vm.pop()

				prototype := vm.peek()

				protoObj, err := allocator.GetObject(prototype.GetHandle())

				if err != nil {
					return value.UNDEFINED, fmt.Errorf("%s was not an object", stringer.String(prototype))
				}

				if p, ok := protoObj.(*native.Prototype); ok {
					p.SetMember(key, method)
				} else {
					return value.UNDEFINED, fmt.Errorf("%s was not an prototype", stringer.String(prototype))
				}
			}
		case chunk.OP_PUSH_PROPERTY:
			{
				v := vm.pop()
				k := vm.pop()

				class := vm.peekN(1)

				classObj, err := allocator.GetObject(class.GetHandle())

				if err != nil {
					return value.UNDEFINED, fmt.Errorf("%s was not an object", stringer.String(class))
				}

				if c, ok := classObj.(*native.ObjClass); ok {
					c.PushProperty(k, v)
				} else {
					return value.UNDEFINED, fmt.Errorf("%s was not an class object", stringer.String(class))
				}
			}
		case chunk.OP_THIS:
			{
				vm.push(f.thisCtx)
			}
		case chunk.OP_YIELD:
			{
				v := native.NewObjectHash()
				v.SetMember(native.KEY_VALUE, vm.pop())
				d := value.FALSE

				if ip >= len(f.fn.ValueChunk().Code) {
					d = value.TRUE
				}

				locals := make([]value.Value, vm.stackTop-f.localStart)
				for i, v := range vm.stack[f.localStart:vm.stackTop] {
					locals[i] = v
					vm.pop()
				}

				f.fn.(*native.ObjGenerator).Ip = ip
				f.fn.(*native.ObjGenerator).Locals = locals

				v.SetMember(native.KEY_DONE, d)

				ip = f.returnIp
				vm.frameCount--
				f = vm.currentFrame()
				c = *f.fn.ValueChunk()
				vm.push(value.EncodeHandle(allocator.Allocate(v)))
			}
		case chunk.OP_IMPORT:
			{
				source := vm.pop()
				src, _ := allocator.GetObject(source.GetHandle())

				str := src.String()
				prev := ROOT_SCRIPT_LOCATION

				if str[0] == '.' {
					str = ROOT_SCRIPT_LOCATION + str[1:]
					ROOT_SCRIPT_LOCATION = strings.Join(strings.Split(str, "/")[:len(strings.Split(str, "/"))-1], "/")
				}

				module, err := compiler.CompileModule(str)

				if err != nil {
					return value.UNDEFINED, fmt.Errorf("failed to parser module %e", err)
				}

				importer := NewVM(vm.debug)

				f, c := importer.Call(module, nil, value.UNDEFINED, 0)
				importer.run(f, c)

				imports[str] = value.EncodeHandle(allocator.Allocate(importer.exports))
				ROOT_SCRIPT_LOCATION = prev
			}
		case chunk.OP_EXPORT:
			{
				k := vm.pop()
				v := vm.pop()

				if v == value.TAG_METHOD_HANDLE {
					v = vm.pop()
					vm.pop() // pop this context
				}

				if vm.exports == nil {
					vm.exports = native.NewObjectHash()
				}

				vm.exports.SetMember(k, v)
			}
		case chunk.OP_NOT:
			{
				arg := vm.pop()
				if truthyValue(arg) {

					vm.push(value.FALSE)
				} else {
					vm.push(value.TRUE)
				}
			}
		case chunk.OP_CREATE_REST_OBJECT:
			{
				origin := vm.pop()
				obj, _ := allocator.GetObject(origin.GetHandle())

				amountOfExludedKeys := c.Code[ip]
				ip++

				exludeMap := map[value.Value]struct{}{}

				for range amountOfExludedKeys {
					key := c.Constants[c.Code[ip]]
					ptr := value.EncodeHandle(allocator.GetPointer(key.GetHandle()))
					exludeMap[ptr] = struct{}{}
					ip++
				}

				newObj := native.NewObjectHash()

				for _, k := range obj.(*native.ObjObject).Keys() {
					if k == native.KEY_PROTO {
						continue
					}
					ptr := value.EncodeHandle(allocator.GetPointer(k.GetHandle()))
					if _, found := exludeMap[ptr]; !found {
						newObj.SetMember(k, obj.(*native.ObjObject).GetMember(k))
					}
				}
				vm.push(value.EncodeHandle(allocator.Allocate(newObj)))
			}
		case chunk.OP_SET_FROM_SPREAD:
			{
				source := vm.pop()
				destination := vm.peek()
				obj, _ := allocator.GetObject(source.GetHandle())

				dstObj, _ := allocator.GetObject(destination.GetHandle())

				for _, k := range obj.(*native.ObjObject).Keys() {
					if k == native.KEY_PROTO {
						continue
					}

					dstObj.(*native.ObjObject).SetMember(k, obj.(*native.ObjObject).GetMember(k))
				}
			}
		}
	}
}

func setHeapScopes(c *value.ValueChunk, heapScope int) error {
	for _, v := range c.Constants {
		if v.IsObject() {
			obj, err := allocator.GetObject(v.GetHandle())

			if err != nil {
				return err
			}

			if obj, ok := obj.(*object.ObjFunction); ok && obj.GetHeapScope() <= heapScope {
				obj.SetHeapScope(heapScope)
				setHeapScopes(obj.ValueChunk(), heapScope)
			}
		}
	}
	return nil
}

func (vm *VM) log(arg value.Value) {
	fmt.Printf("%s\n", stringer.String(arg))
}
