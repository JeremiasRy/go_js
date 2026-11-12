package heap

import (
	"go_js/object"
	"go_js/value"
	"testing"
)

type testObject struct {
	id     int
	marked bool
}

func (o *testObject) Type() object.ObjType {
	return object.OBJ_OBJECT
}

func (o *testObject) String() string {
	return "test object"
}

func (o *testObject) Mark() {
	o.marked = true
}

func (o *testObject) Marked() bool {
	return o.marked
}

func (o *testObject) Clear() {
	o.marked = false
}

func (o *testObject) GetReferencingValues() []value.Value {
	return nil
}

func TestShouldDefragment(t *testing.T) {
	tests := []struct {
		name    string
		objects int
		free    int
		want    bool
	}{
		{name: "small heap", objects: 100, free: 90, want: false},
		{name: "few free slots", objects: 1000, free: 63, want: false},
		{name: "below ratio", objects: 128, free: 63, want: false},
		{name: "at ratio", objects: 128, free: 64, want: true},
		{name: "fragmented", objects: 1000, free: 600, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &Heap{
				objects:  make([]object.Object, test.objects),
				freeList: make(map[uint32]struct{}, test.free),
			}
			for i := 0; i < test.free; i++ {
				h.freeList[uint32(i)] = struct{}{}
			}
			if got := h.shouldDefragment(); got != test.want {
				t.Fatalf("shouldDefragment() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDefragmentCompactsAndRemaps(t *testing.T) {
	objects := []*testObject{
		newTestObject(0),
		newTestObject(1),
		newTestObject(2),
		newTestObject(3),
		newTestObject(4),
	}
	h := &Heap{
		objects: []object.Object{
			objects[0],
			objects[1],
			objects[2],
			objects[3],
			objects[4],
		},
		freeList: map[uint32]struct{}{
			1: {},
			3: {},
		},
		handleLayer: map[uint32]uint32{
			10: 0,
			11: 1,
			12: 2,
			13: 3,
			14: 4,
			15: 99,
		},
		strings: map[string]uint32{
			"zero": 0,
			"one":  1,
			"two":  2,
			"gone": 3,
		},
	}

	h.defragment()

	if len(h.objects) != 3 {
		t.Fatalf("compacted object count = %d, want 3", len(h.objects))
	}
	if h.objects[0] != objects[0] || h.objects[1] != objects[2] || h.objects[2] != objects[4] {
		t.Fatal("live objects were not compacted in order")
	}
	if len(h.freeList) != 0 {
		t.Fatalf("free list contains %d slots after defragment", len(h.freeList))
	}

	wantHandles := map[uint32]uint32{
		10: 0,
		12: 1,
		14: 2,
	}
	if len(h.handleLayer) != len(wantHandles) {
		t.Fatalf("handle count = %d, want %d", len(h.handleLayer), len(wantHandles))
	}
	for handle, want := range wantHandles {
		got, found := h.handleLayer[handle]
		if !found || got != want {
			t.Fatalf("handle %d maps to %d (found %v), want %d", handle, got, found, want)
		}
	}

	wantStrings := map[string]uint32{
		"zero": 0,
		"two":  1,
	}
	if len(h.strings) != len(wantStrings) {
		t.Fatalf("string count = %d, want %d", len(h.strings), len(wantStrings))
	}
	for str, want := range wantStrings {
		got, found := h.strings[str]
		if !found || got != want {
			t.Fatalf("string %q maps to %d (found %v), want %d", str, got, found, want)
		}
	}
}

func newTestObject(id int) *testObject {
	return &testObject{id: id}
}
