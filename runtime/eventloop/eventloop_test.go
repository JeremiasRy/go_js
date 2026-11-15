package eventloop

import (
	"go_js/object"
	"go_js/trace"
	"sync"
	"testing"
	"time"
)

type testWorkJob struct {
	item    trace.WorkItem
	release chan struct{}
}

func (j *testWorkJob) Work(_ chan *object.JobChannelMessage, done func()) {
	<-j.release
	done()
}

func (j *testWorkJob) TraceWorkItem() trace.WorkItem {
	return j.item
}

func TestSnapshotTracksOutstandingWorkItems(t *testing.T) {
	var wg sync.WaitGroup
	Init(&wg)
	Start()

	first := &testWorkJob{
		item:    trace.WorkItem{Kind: "timer", Label: "setTimeout(100ms) callback", DelayMS: 100},
		release: make(chan struct{}),
	}
	second := &testWorkJob{
		item:    trace.WorkItem{Kind: "async", Label: "resume await in fetchOne", Source: "await"},
		release: make(chan struct{}),
	}
	Dispatch(first)
	Dispatch(second)

	deadline := time.Now().Add(2 * time.Second)
	for {
		snapshot := SnapshotState()
		if len(snapshot.Items) == 2 {
			if snapshot.Work != 2 {
				t.Fatalf("expected 2 outstanding jobs, got %d", snapshot.Work)
			}
			if snapshot.Items[0].Label != first.item.Label || snapshot.Items[1].Label != second.item.Label {
				t.Fatalf("unexpected work items: %+v", snapshot.Items)
			}
			if snapshot.Items[0].ID == snapshot.Items[1].ID {
				t.Fatalf("expected unique work item IDs, got %q", snapshot.Items[0].ID)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for work items: %+v", snapshot)
		}
		time.Sleep(time.Millisecond)
	}

	close(first.release)
	close(second.release)
	wg.Wait()

	snapshot := SnapshotState()
	if snapshot.Work != 0 || len(snapshot.Items) != 0 {
		t.Fatalf("expected finished work to be cleared, got %+v", snapshot)
	}
}
