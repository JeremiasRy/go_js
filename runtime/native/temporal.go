package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"time"
)

type ObjTemporalInstant struct {
	ObjObject
	Nanoseconds int64
}

func NewTemporalInstant(nanoseconds int64) *ObjTemporalInstant {
	instant := &ObjTemporalInstant{Nanoseconds: nanoseconds}
	instant.Members = map[string]ObjectValueEntry{}
	instant.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	instant.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("toString"))), hostFunction("toString", HOST_TEMPORAL_INSTANT_TO_STRING))
	instant.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("epochMilliseconds"))), hostFunction("epochMilliseconds", HOST_TEMPORAL_INSTANT_EPOCH_MILLISECONDS))
	instant.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("epochNanoseconds"))), hostFunction("epochNanoseconds", HOST_TEMPORAL_INSTANT_EPOCH_NANOSECONDS))
	instant.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("valueOf"))), hostFunction("valueOf", HOST_TEMPORAL_INSTANT_EPOCH_NANOSECONDS))
	return instant
}

func (*ObjTemporalInstant) Type() object.ObjType { return object.OBJ_OBJECT }
func (*ObjTemporalInstant) String() string       { return "Temporal.Instant" }

type TemporalInstantConstructor struct {
	ObjObject
}

func NewTemporalInstantConstructor() *TemporalInstantConstructor {
	ctor := &TemporalInstantConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	ctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("from"))), hostFunction("from", HOST_TEMPORAL_INSTANT_FROM))
	ctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("fromEpochMilliseconds"))), hostFunction("fromEpochMilliseconds", HOST_TEMPORAL_INSTANT_FROM_EPOCH_MILLISECONDS))
	return ctor
}

func (*TemporalInstantConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*TemporalInstantConstructor) String() string       { return "function Instant" }

func NewTemporalObject() *ObjObject {
	temporal := newHostObject()
	now := newHostObject()
	now.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("instant"))), hostFunction("instant", HOST_TEMPORAL_NOW))
	temporal.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("Now"))), value.EncodeHandle(heap.Allocate(now)))
	temporal.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("Instant"))), value.EncodeHandle(heap.Allocate(NewTemporalInstantConstructor())))
	return temporal
}

func ParseTemporalInstant(text string) (int64, error) {
	parsed, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return 0, err
	}
	return parsed.UnixNano(), nil
}
