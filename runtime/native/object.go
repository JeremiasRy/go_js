package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"sort"
	"strconv"
)

func objectAsObjObject(obj object.Object) (*ObjObject, bool) {
	switch o := obj.(type) {
	case *ObjObject:
		return o, true
	case *ObjClass:
		return &o.ObjObject, true
	case *Instance:
		return &o.ObjObject, true
	case *Prototype:
		return &o.ObjObject, true
	case *ObjModuleNamespace:
		return &o.ObjObject, true
	}
	return nil, false
}

type ObjectValueEntry struct {
	Value           value.Value
	init            int
	Get             value.Value
	Set             value.Value
	accessor        bool
	nonWritable     bool
	nonEnumerable   bool
	nonConfigurable bool
}

func (entry ObjectValueEntry) IsAccessor() bool {
	return entry.accessor
}

func (entry ObjectValueEntry) GetInit() int {
	return entry.init
}

func (entry ObjectValueEntry) Writable() bool {
	return !entry.nonWritable
}

func (entry ObjectValueEntry) Enumerable() bool {
	return !entry.nonEnumerable
}

func (entry ObjectValueEntry) Configurable() bool {
	return !entry.nonConfigurable
}

type ObjObject struct {
	marked        bool
	init          int
	nonExtensible bool
	Members       map[string]ObjectValueEntry
}

func (o *ObjObject) Extensible() bool {
	return !o.nonExtensible
}

func (o *ObjObject) SetExtensible(extensible bool) {
	o.nonExtensible = !extensible
}

func (o *ObjObject) Seal() {
	o.nonExtensible = true
	for key, entry := range o.Members {
		entry.nonConfigurable = true
		o.Members[key] = entry
	}
}

func (o *ObjObject) Freeze() {
	o.Seal()
	for key, entry := range o.Members {
		if !entry.accessor {
			entry.nonWritable = true
		}
		o.Members[key] = entry
	}
}

func (o *ObjObject) IsSealed() bool {
	if o.nonExtensible {
		for _, entry := range o.Members {
			if entry.Configurable() {
				return false
			}
		}
		return true
	}
	return false
}

func (o *ObjObject) IsFrozen() bool {
	if o.IsSealed() {
		for _, entry := range o.Members {
			if !entry.accessor && entry.Writable() {
				return false
			}
		}
		return true
	}
	return false
}

func (o *ObjObject) GetOwnEntry(k value.Value) (ObjectValueEntry, bool) {
	key, ok := objectKeyString(k)
	if !ok {
		return ObjectValueEntry{}, false
	}
	entry, found := o.Members[key]
	return entry, found
}

func (o *ObjObject) SetAccessor(k, getter, setter value.Value) {
	key, ok := objectKeyString(k)
	if !ok {
		return
	}
	entry, found := o.Members[key]
	if !found {
		entry.init = o.nextInit()
		entry.Get = value.UNDEFINED
		entry.Set = value.UNDEFINED
	}
	entry.accessor = true
	if getter != value.UNDEFINED {
		entry.Get = getter
	}
	if setter != value.UNDEFINED {
		entry.Set = setter
	}
	o.Members[key] = entry
}

func (o *ObjObject) PrototypeValue() value.Value {
	if entry, ok := o.Members[PROTOTYPE_PROPERTY_STRING]; ok {
		return entry.Value
	}
	return value.NULL
}

func (o *ObjObject) SetPrototypeValue(proto value.Value) {
	key := PROTOTYPE_PROPERTY_STRING
	entry, found := o.Members[key]
	if !found {
		entry.init = o.nextInit()
	}
	entry.Value = proto
	o.Members[key] = entry
}

func (o *ObjObject) nextInit() int {
	o.init++
	return o.init
}

func objectKeyString(k value.Value) (string, bool) {
	if k.IsObject() {
		obj, err := heap.GetObject(k.GetHandle())
		if err != nil {
			return "", false
		}
		switch obj := obj.(type) {
		case *LightString:
			return obj.String(), true
		case *ObjString:
			return obj.String(), true
		case *ObjStringBuilder:
			return obj.String(), true
		case *ObjSymbol:
			return obj.Key, true
		}
	}
	if k.IsNumber() {
		return strconv.FormatFloat(k.AsNumber(), 'f', -1, 64), true
	}
	return "", false
}

func (o *ObjObject) Delete(k value.Value) bool {
	key, ok := objectKeyString(k)
	if !ok {
		return true
	}
	if entry, found := o.Members[key]; found {
		if !entry.Configurable() {
			return false
		}
		delete(o.Members, key)
	}
	return true
}

func (o *ObjObject) GetReferencingValues() []value.Value {
	arr := []value.Value{}
	for _, v := range o.Members {
		if v.Value.IsObject() {
			arr = append(arr, v.Value)
		}
		if v.Get.IsObject() {
			arr = append(arr, v.Get)
		}
		if v.Set.IsObject() {
			arr = append(arr, v.Set)
		}
	}
	return arr
}

func (o *ObjObject) Mark() {
	o.marked = true
}

func (o *ObjObject) Marked() bool {
	return o.marked
}

func (o *ObjObject) Clear() {
	o.marked = false
}

func NewNullPrototypeObject() *ObjObject {
	return &ObjObject{Members: map[string]ObjectValueEntry{}}
}

func NewObjectHash() *ObjObject {
	o := &ObjObject{
		init:    0,
		Members: map[string]ObjectValueEntry{},
	}
	o.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return o
}

func NewObjectFromObject(parent value.Value) *ObjObject {
	o := &ObjObject{
		init:    0,
		Members: map[string]ObjectValueEntry{},
	}

	o.SetMember(KEY_PROTO, parent)
	return o
}

func (*ObjObject) Type() object.ObjType {
	return object.OBJ_OBJECT
}

func (oh *ObjObject) String() string {
	return "[Object object]"
}

func (o *ObjObject) Keys() []value.Value {
	keys := []value.Value{}
	seen := map[string]bool{}

	appendKeys := func(target *ObjObject) {
		for _, item := range orderedMemberEntries(target) {
			if len(item.Key) > 0 && item.Key[0] == 0 {
				continue
			}
			if seen[item.Key] {
				continue
			}
			seen[item.Key] = true
			if !item.Entry.Enumerable() {
				continue
			}
			keys = append(keys, value.EncodeHandle(heap.Allocate(NewLightString(item.Key))))
		}
	}

	appendKeys(o)
	p := value.NULL
	if entry, found := o.Members[PROTOTYPE_PROPERTY_STRING]; found {
		p = entry.Value
	}
	for p != value.NULL && p.IsObject() {
		obj, _ := heap.GetObject(p.GetHandle())
		if proto, ok := obj.(*ObjObject); ok {
			appendKeys(proto)
		}
		if obj, ok := obj.(object.Hashable); ok {
			p = obj.GetMember(KEY_PROTO)
		} else {
			p = value.NULL
		}
	}
	return keys
}

func (o *ObjObject) Values() []value.Value {
	panic("unimplemented")
}

type ObjectHasOwn struct {
	ObjNativeFn
}

func NewObjectHasOwn() *ObjectHasOwn {
	o := &ObjectHasOwn{}
	o.name = "hasOwn"
	return o
}

func (*ObjectHasOwn) HasOwn(target value.Value, prop value.Value) value.Value {
	if !target.IsObject() {
		return value.FALSE
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return value.FALSE
	}
	if hashable, ok := obj.(object.Hashable); ok && hashable.HasOwn(prop) {
		return value.TRUE
	}
	return value.FALSE
}

type ObjectSetPrototypeOf struct {
	ObjNativeFn
}

func NewObjectSetPrototypeOf() *ObjectSetPrototypeOf {
	o := &ObjectSetPrototypeOf{}
	o.name = "setPrototypeOf"
	return o
}

type ObjectGetPrototypeOf struct {
	ObjNativeFn
}

func NewObjectGetPrototypeOf() *ObjectGetPrototypeOf {
	o := &ObjectGetPrototypeOf{}
	o.name = "getPrototypeOf"
	return o
}

func (*ObjectGetPrototypeOf) GetPrototypeOf(target value.Value) value.Value {
	if !target.IsObject() {
		return value.NULL
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return value.NULL
	}
	if o, ok := obj.(interface{ PrototypeValue() value.Value }); ok {
		return o.PrototypeValue()
	}
	return value.NULL
}

func (*ObjectSetPrototypeOf) SetPrototypeOf(target, proto value.Value) value.Value {
	if !target.IsObject() {
		return value.UNDEFINED
	}
	if proto != value.NULL && !proto.IsObject() {
		return value.UNDEFINED
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return value.UNDEFINED
	}
	o, ok := obj.(interface {
		SetPrototypeValue(value.Value)
		PrototypeValue() value.Value
		Extensible() bool
	})
	if !ok {
		return value.UNDEFINED
	}
	if !o.Extensible() && o.PrototypeValue() != proto {
		return value.UNDEFINED
	}
	current := proto
	for current.IsObject() {
		if heap.GetPointer(current.GetHandle()) == heap.GetPointer(target.GetHandle()) {
			return value.UNDEFINED
		}
		currentObj, err := heap.GetObject(current.GetHandle())
		if err != nil {
			break
		}
		p, ok := currentObj.(interface{ PrototypeValue() value.Value })
		if !ok {
			break
		}
		current = p.PrototypeValue()
	}
	o.SetPrototypeValue(proto)
	return target
}

type ObjectValueOf struct {
	ObjNativeFn
}

func NewObjectValueOf() *ObjectValueOf {
	o := &ObjectValueOf{}
	o.name = "valueOf"
	return o
}

type ObjectGetOwnPropertySymbols struct {
	ObjNativeFn
}

func NewObjectGetOwnPropertySymbols() *ObjectGetOwnPropertySymbols {
	o := &ObjectGetOwnPropertySymbols{}
	o.name = "getOwnPropertySymbols"
	return o
}

type ObjectGetOwnPropertyNames struct {
	ObjNativeFn
}

func NewObjectGetOwnPropertyNames() *ObjectGetOwnPropertyNames {
	o := &ObjectGetOwnPropertyNames{}
	o.name = "getOwnPropertyNames"
	return o
}

type ObjectDefineProperty struct {
	ObjNativeFn
}

func NewObjectDefineProperty() *ObjectDefineProperty {
	o := &ObjectDefineProperty{}
	o.name = "defineProperty"
	return o
}

type ObjectGetOwnPropertyDescriptor struct {
	ObjNativeFn
}

func NewObjectGetOwnPropertyDescriptor() *ObjectGetOwnPropertyDescriptor {
	o := &ObjectGetOwnPropertyDescriptor{}
	o.name = "getOwnPropertyDescriptor"
	return o
}

func (*ObjectGetOwnPropertySymbols) Symbols(target value.Value) []value.Value {
	result := []value.Value{}
	if !target.IsObject() {
		return result
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return result
	}
	o, ok := objectAsObjObject(obj)
	if !ok {
		return result
	}
	for _, item := range orderedMemberEntries(o) {
		if len(item.Key) > 0 && item.Key[0] == 0 {
			if symbol, found := SymbolByKey(item.Key); found {
				result = append(result, symbol)
			}
		}
	}
	return result
}

func (*ObjectGetOwnPropertyNames) Names(target value.Value) []value.Value {
	result := []value.Value{}
	if !target.IsObject() {
		return result
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return result
	}
	o, ok := objectAsObjObject(obj)
	if !ok {
		return result
	}
	for _, item := range orderedMemberEntries(o) {
		if len(item.Key) > 0 && item.Key[0] == 0 {
			continue
		}
		result = append(result, value.EncodeHandle(heap.Allocate(NewLightString(item.Key))))
	}
	return result
}

func (*ObjectDefineProperty) Define(target, key, descriptor value.Value) value.Value {
	if !target.IsObject() || !descriptor.IsObject() {
		return value.UNDEFINED
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return value.UNDEFINED
	}
	o, ok := obj.(*ObjObject)
	if !ok {
		if arr, isArr := obj.(*ObjArr); isArr {
			o = &arr.ObjObject
		} else {
			return value.UNDEFINED
		}
	}
	descObj, err := heap.GetObject(descriptor.GetHandle())
	if err != nil {
		return value.UNDEFINED
	}
	desc, ok := descObj.(*ObjObject)
	if !ok {
		return value.UNDEFINED
	}
	keyString, ok := objectKeyString(key)
	if !ok {
		return value.UNDEFINED
	}
	entry, found := o.Members[keyString]
	if !found {
		if !o.Extensible() {
			return value.UNDEFINED
		}
		entry.init = o.nextInit()
		entry.nonWritable = true
		entry.nonEnumerable = true
		entry.nonConfigurable = true
	}
	getValue := func(name string) (value.Value, bool) {
		k := value.EncodeHandle(heap.Allocate(NewLightString(name)))
		if !desc.HasOwn(k) {
			return value.UNDEFINED, false
		}
		return desc.GetMember(k), true
	}
	valueField, hasValue := getValue("value")
	writableField, hasWritable := getValue("writable")
	enumerableField, hasEnumerable := getValue("enumerable")
	configurableField, hasConfigurable := getValue("configurable")
	getField, hasGet := getValue("get")
	setField, hasSet := getValue("set")

	if hasValue && (hasGet || hasSet) {
		return value.UNDEFINED
	}
	if found && !entry.Configurable() {
		if hasConfigurable && truthy(configurableField) {
			return value.UNDEFINED
		}
		if hasEnumerable && truthy(enumerableField) != entry.Enumerable() {
			return value.UNDEFINED
		}
		if entry.IsAccessor() {
			if hasValue || hasWritable {
				return value.UNDEFINED
			}
			if hasGet && !ObjectIsSame(getField, entry.Get) {
				return value.UNDEFINED
			}
			if hasSet && !ObjectIsSame(setField, entry.Set) {
				return value.UNDEFINED
			}
		} else {
			if hasGet || hasSet {
				return value.UNDEFINED
			}
			if hasWritable && truthy(writableField) && !entry.Writable() {
				return value.UNDEFINED
			}
			if hasValue && !entry.Writable() && !ObjectIsSame(valueField, entry.Value) {
				return value.UNDEFINED
			}
		}
	}
	if hasValue {
		entry.Value = valueField
		entry.accessor = false
	}
	if hasWritable {
		entry.nonWritable = !truthy(writableField)
	}
	if hasEnumerable {
		entry.nonEnumerable = !truthy(enumerableField)
	}
	if hasConfigurable {
		entry.nonConfigurable = !truthy(configurableField)
	}
	if hasGet {
		entry.Get = getField
		entry.accessor = true
	}
	if hasSet {
		entry.Set = setField
		entry.accessor = true
	}
	o.Members[keyString] = entry
	return target
}

func (*ObjectGetOwnPropertyDescriptor) Descriptor(target, key value.Value) value.Value {
	if !target.IsObject() {
		return value.UNDEFINED
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return value.UNDEFINED
	}
	if fn, ok := obj.(*object.ObjFunction); ok {
		keyString, ok := objectKeyString(key)
		if !ok {
			return value.UNDEFINED
		}
		desc := NewObjectHash()
		switch keyString {
		case "prototype":
			if fn.IsArrow || fn.IsMethod {
				return value.UNDEFINED
			}
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("value"))), EnsureFunctionPrototype(fn, target))
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("writable"))), value.TRUE)
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("enumerable"))), value.FALSE)
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("configurable"))), value.FALSE)
			return value.EncodeHandle(heap.Allocate(desc))
		case "name":
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("value"))), value.EncodeHandle(heap.Allocate(NewLightString(fn.Name))))
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("writable"))), value.FALSE)
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("enumerable"))), value.FALSE)
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("configurable"))), value.TRUE)
			return value.EncodeHandle(heap.Allocate(desc))
		case "length":
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("value"))), value.ValueFromFloat64(float64(fn.Length)))
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("writable"))), value.FALSE)
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("enumerable"))), value.FALSE)
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("configurable"))), value.TRUE)
			return value.EncodeHandle(heap.Allocate(desc))
		}
		if prop, found := fn.Props[keyString]; found {
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("value"))), prop)
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("writable"))), value.TRUE)
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("enumerable"))), value.TRUE)
			desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("configurable"))), value.TRUE)
			return value.EncodeHandle(heap.Allocate(desc))
		}
		return value.UNDEFINED
	}
	o, ok := objectAsObjObject(obj)
	if !ok {
		return value.UNDEFINED
	}
	entry, found := o.GetOwnEntry(key)
	if !found {
		return value.UNDEFINED
	}
	desc := NewObjectHash()
	desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("enumerable"))), boolValueValue(entry.Enumerable()))
	desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("configurable"))), boolValueValue(entry.Configurable()))
	if entry.IsAccessor() {
		desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("get"))), entry.Get)
		desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("set"))), entry.Set)
	} else {
		desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("value"))), entry.Value)
		desc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("writable"))), boolValueValue(entry.Writable()))
	}
	return value.EncodeHandle(heap.Allocate(desc))
}

func boolValueValue(b bool) value.Value {
	if b {
		return value.TRUE
	}
	return value.FALSE
}

func truthy(v value.Value) bool {
	if v.IsBoolean() {
		return v == value.TRUE
	}
	if v.IsNumber() {
		return v.AsNumber() != 0
	}
	if v == value.NULL || v == value.UNDEFINED {
		return false
	}
	return true
}

type ToString struct {
	ObjNativeFn
}

func NewToString() *ToString {
	ts := &ToString{}
	ts.name = "toString"

	return ts
}

type Create struct {
	ObjNativeFn
}

func NewCreate() *Create {
	c := &Create{}
	c.name = "create"

	return c
}

type HasOwnProperty struct {
	ObjNativeFn
}

func NewHasOwnProperty() *HasOwnProperty {
	c := &HasOwnProperty{}
	c.name = "hasOwnProperty"

	return c
}

func (*HasOwnProperty) HasOwn(obj value.Value, prop value.Value) bool {
	o, err := heap.GetObject(obj.GetHandle())
	if err != nil {
		return false
	}
	if fn, ok := o.(*object.ObjFunction); ok {
		key, ok := objectKeyString(prop)
		if !ok {
			return false
		}
		switch key {
		case "prototype":
			if fn.IsArrow || fn.IsMethod {
				return false
			}
			EnsureFunctionPrototype(fn, obj)
			return true
		case "name", "length":
			return true
		}
		if _, found := fn.Props[key]; found {
			return true
		}
		return false
	}
	if hashable, ok := o.(object.Hashable); ok {
		return hashable.HasOwn(prop)
	}
	return false
}

func (ts *ToString) ToString(obj object.Object) string {
	return obj.String()
}

func (obj *ObjObject) HasOwn(prop value.Value) bool {
	key, ok := objectKeyString(prop)
	if !ok {
		return false
	}
	_, found := obj.Members[key]
	return found
}

func (obj *ObjObject) GetMember(k value.Value) value.Value {
	key, ok := objectKeyString(k)
	if !ok {
		return value.UNDEFINED
	}
	if entry, found := obj.Members[key]; found {
		if entry.accessor {
			return entry.Get
		}
		return entry.Value
	}
	proto := obj.PrototypeValue()
	if proto.IsType(value.NULL) || !proto.IsObject() {
		return value.UNDEFINED
	}
	protoObj, err := heap.GetObject(proto.GetHandle())
	if err != nil {
		return value.UNDEFINED
	}
	if protoObj, ok := protoObj.(object.Hashable); ok {
		return protoObj.GetMember(k)
	}
	return value.UNDEFINED
}

func (obj *ObjObject) SetMember(k, v value.Value) {
	key, ok := objectKeyString(k)
	if !ok {
		return
	}
	if entry, found := obj.Members[key]; found {
		if entry.accessor {
			return
		}
		entry.Value = v
		obj.Members[key] = entry
		return
	}
	if obj.nonExtensible {
		return
	}
	obj.Members[key] = obj.NewValueEntry(v)
}

func (obj *ObjObject) DefineNonConfigurableMember(k, v value.Value) {
	key, ok := objectKeyString(k)
	if !ok {
		return
	}
	entry := obj.NewValueEntry(v)
	entry.nonConfigurable = true
	obj.Members[key] = entry
}

func (obj *ObjObject) NewValueEntry(v value.Value) ObjectValueEntry {
	obj.init++
	return ObjectValueEntry{Value: v, init: obj.init}
}

type ObjectValues struct {
	ObjNativeFn
}

func NewObjectValues() *ObjectValues {
	ov := &ObjectValues{}
	ov.name = "values"
	return ov
}

func orderedMemberEntries(obj *ObjObject) []struct {
	Key   string
	Entry ObjectValueEntry
} {
	entries := []struct {
		Key   string
		Entry ObjectValueEntry
	}{}
	for key, entry := range obj.Members {
		if key == PROTOTYPE_PROPERTY_STRING {
			continue
		}
		entries = append(entries, struct {
			Key   string
			Entry ObjectValueEntry
		}{key, entry})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		iIndex, iIsIndex := canonicalArrayIndex(entries[i].Key)
		jIndex, jIsIndex := canonicalArrayIndex(entries[j].Key)
		if iIsIndex && jIsIndex {
			return iIndex < jIndex
		}
		if iIsIndex != jIsIndex {
			return iIsIndex
		}
		return entries[i].Entry.init < entries[j].Entry.init
	})
	return entries
}

func canonicalArrayIndex(key string) (uint64, bool) {
	if key == "" {
		return 0, false
	}
	if key == "0" {
		return 0, true
	}
	if key[0] == '0' {
		return 0, false
	}
	var value uint64
	for i := 0; i < len(key); i++ {
		if key[i] < '0' || key[i] > '9' {
			return 0, false
		}
		value = value*10 + uint64(key[i]-'0')
		if value >= 4294967295 {
			return 0, false
		}
	}
	return value, true
}

func (*ObjectValues) Values(o value.Value) []value.Value {
	if o.IsObject() {
		obj, err := heap.GetObject(o.GetHandle())

		if err != nil {
			panic("couldn't receive object from heap")
		}

		switch obj := obj.(type) {
		case *LightString:
			r := []value.Value{}
			for _, unit := range UTF16Units(obj.String()) {
				r = append(r, value.EncodeHandle(heap.Allocate(NewLightString(UTF16FromUnits([]uint16{unit})))))
			}
			return r
		case *ObjString:
			r := []value.Value{}
			for _, unit := range UTF16Units(obj.String()) {
				r = append(r, value.EncodeHandle(heap.Allocate(NewLightString(UTF16FromUnits([]uint16{unit})))))
			}
			return r
		case *ObjObject:
			{
				r := []value.Value{}
				for _, item := range orderedMemberEntries(obj) {
					if len(item.Key) > 0 && item.Key[0] == 0 {
						continue
					}
					if item.Entry.Enumerable() {
						r = append(r, item.Entry.Value)
					}
				}
				return r
			}
		case *ObjClass, *Instance, *Prototype, *ObjModuleNamespace:
			{
				o, _ := objectAsObjObject(obj)
				r := []value.Value{}
				for _, item := range orderedMemberEntries(o) {
					if len(item.Key) > 0 && item.Key[0] == 0 {
						continue
					}
					if item.Entry.Enumerable() {
						r = append(r, item.Entry.Value)
					}
				}
				return r
			}
		case *ObjArr:
			r := []value.Value{}
			for i := 0; i < obj.Length(); i++ {
				if obj.HasIndex(i) {
					r = append(r, obj.GetElementAt(i))
				}
			}
			return r
		}
	}
	return []value.Value{}
}

type ObjectKeys struct {
	ObjNativeFn
}

func NewObjectKeys() *ObjectKeys {
	ok := &ObjectKeys{}
	ok.name = "keys"
	return ok
}

func (*ObjectKeys) Keys(o value.Value) []value.Value {
	r := []value.Value{}
	if o.IsObject() {
		obj, err := heap.GetObject(o.GetHandle())

		if err != nil {
			panic("couldn't receive object from heap")
		}

		switch obj := obj.(type) {
		case *LightString:
			r := []value.Value{}
			for i := range UTF16Units(obj.String()) {
				r = append(r, value.EncodeHandle(heap.Allocate(NewLightString(strconv.Itoa(i)))))
			}
			return r
		case *ObjString:
			r := []value.Value{}
			for i := range UTF16Units(obj.String()) {
				r = append(r, value.EncodeHandle(heap.Allocate(NewLightString(strconv.Itoa(i)))))
			}
			return r
		case *ObjObject:
			{
				r := []value.Value{}
				for _, item := range orderedMemberEntries(obj) {
					if len(item.Key) > 0 && item.Key[0] == 0 {
						continue
					}
					if item.Entry.Enumerable() {
						val := value.EncodeHandle(heap.Allocate(NewLightString(item.Key)))
						r = append(r, val)
					}
				}
				return r
			}
		case *ObjClass, *Instance, *Prototype, *ObjModuleNamespace:
			{
				o, _ := objectAsObjObject(obj)
				r := []value.Value{}
				for _, item := range orderedMemberEntries(o) {
					if len(item.Key) > 0 && item.Key[0] == 0 {
						continue
					}
					if item.Entry.Enumerable() {
						val := value.EncodeHandle(heap.Allocate(NewLightString(item.Key)))
						r = append(r, val)
					}
				}
				return r
			}
		case *ObjArr:
			for i := 0; i < obj.Length(); i++ {
				if obj.HasIndex(i) {
					r = append(r, value.EncodeHandle(heap.Allocate(NewLightString(strconv.Itoa(i)))))
				}
			}
			return r
		}
	}
	return r
}

func OwnPropertyKeys(target value.Value) []value.Value {
	result := []value.Value{}
	if !target.IsObject() {
		return result
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return result
	}
	o, ok := objectAsObjObject(obj)
	if !ok {
		return result
	}
	for _, item := range orderedMemberEntries(o) {
		if len(item.Key) > 0 && item.Key[0] == 0 {
			if symbol, found := SymbolByKey(item.Key); found {
				result = append(result, symbol)
			}
			continue
		}
		result = append(result, value.EncodeHandle(heap.Allocate(NewLightString(item.Key))))
	}
	return result
}

type IsPrototypeOf struct {
	ObjNativeFn
}

func NewIsPrototypeOf() *IsPrototypeOf {
	i := &IsPrototypeOf{}
	i.name = "isPrototypeOf"
	return i
}

type PropertyIsEnumerable struct {
	ObjNativeFn
}

func NewPropertyIsEnumerable() *PropertyIsEnumerable {
	p := &PropertyIsEnumerable{}
	p.name = "propertyIsEnumerable"
	return p
}

func ObjectToStringTag(v value.Value) string {
	if v.IsObject() {
		obj, err := heap.GetObject(v.GetHandle())
		if err == nil {
			switch obj.(type) {
			case *ObjArr:
				return "[object Array]"
			case *ObjRegExp:
				return "[object RegExp]"
			case *DateObject:
				return "[object Date]"
			case *ObjError:
				return "[object Error]"
			case *Map:
				return "[object Map]"
			case *Set:
				return "[object Set]"
			case *ObjPromise:
				return "[object Promise]"
			case *object.ObjFunction, *ObjAsyncFunction, *ObjGenerator, *ObjBoundFunction, *ObjNativeFn:
				return "[object Function]"
			case *ObjSymbol:
				return "[object Symbol]"
			case *LightString, *ObjString, *ObjStringBuilder:
				return "[object String]"
			case *BoxedPrimitive:
				return ObjectToStringTag(obj.(*BoxedPrimitive).Primitive)
			}
		}
		return "[object Object]"
	}
	if v == value.UNDEFINED {
		return "[object Undefined]"
	}
	if v == value.NULL {
		return "[object Null]"
	}
	if v.IsBoolean() {
		return "[object Boolean]"
	}
	if v.IsNumber() {
		return "[object Number]"
	}
	return "[object Object]"
}
