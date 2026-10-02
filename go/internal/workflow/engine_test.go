package workflow

import (
	"sync"
	"testing"
)

func must(t *testing.T, e *Engine, a string, p Record, role string) Record {
	t.Helper()
	r, err := e.Command(a, p, role)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func status(t *testing.T, expected int, err error) {
	t.Helper()
	p, ok := err.(Problem)
	if !ok || p.Status != expected {
		t.Fatalf("expected %d, got %v", expected, err)
	}
}
func TestBoundedQueueDedupAndFailureReplay(t *testing.T) {
	e := New()
	p := Record{"id": "a", "kind": "order", "quantity": 2}
	must(t, e, "enqueue", p, "learner")
	must(t, e, "enqueue", p, "learner")
	for _, id := range []string{"b", "c", "d"} {
		must(t, e, "enqueue", Record{"id": id, "kind": "order", "quantity": 2}, "learner")
	}
	_, err := e.Command("enqueue", Record{"id": "e", "kind": "order", "quantity": 2}, "learner")
	status(t, 429, err)
	must(t, e, "drain", Record{"simulateFailure": true}, "operator")
	if e.State()["queueDepth"] != 4 {
		t.Fatal("failed events must be retained")
	}
	must(t, e, "drain", Record{}, "operator")
	if e.State()["queueDepth"] != 0 {
		t.Fatal("queue should drain")
	}
	_, err = e.Command("enqueue", Record{"id": "a", "kind": "order", "quantity": 3}, "learner")
	status(t, 409, err)
}
func TestConcurrentEnqueue(t *testing.T) {
	e := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.Command("enqueue", Record{"id": "same", "kind": "order", "quantity": 2}, "learner")
		}()
	}
	wg.Wait()
	if e.State()["queueDepth"] != 1 {
		t.Fatal("duplicate event queued more than once")
	}
}
func TestSchedulingConflictTenantAndCancellation(t *testing.T) {
	e := New()
	p := Record{"id": "a", "tenant": "clinic-a", "resource": "Room1", "start": 2000000000, "end": 2000001800}
	must(t, e, "book", p, "clinic-a")
	must(t, e, "book", p, "clinic-a")
	_, err := e.Command("book", Record{"id": "b", "tenant": "clinic-a", "resource": "Room1", "start": 2000000100, "end": 2000001900}, "clinic-a")
	status(t, 409, err)
	_, err = e.Command("cancel", Record{"id": "a"}, "clinic-b")
	status(t, 403, err)
	must(t, e, "cancel", Record{"id": "a"}, "clinic-a")
	must(t, e, "book", Record{"id": "b", "tenant": "clinic-a", "resource": "Room1", "start": 2000000100, "end": 2000001900}, "clinic-a")
}
func TestAssessmentPermissionAndStaleApproval(t *testing.T) {
	e := New()
	p := Record{"id": "s", "student": "S", "answers": []any{"transaction", "wrong", "audit"}, "deadline": 2000000000}
	r := must(t, e, "submit", p, "learner")
	if r["score"] != float64(2) {
		t.Fatal(r)
	}
	_, err := e.Command("submit", p, "learner")
	status(t, 409, err)
	_, err = e.Command("feedback", Record{"id": "s", "version": 0, "feedback": "Review rubric"}, "learner")
	status(t, 403, err)
	must(t, e, "feedback", Record{"id": "s", "version": 0, "feedback": "Review rubric"}, "instructor")
	_, err = e.Command("publish", Record{"id": "s", "version": 0}, "instructor")
	status(t, 409, err)
	r = must(t, e, "publish", Record{"id": "s", "version": 1}, "instructor")
	if r["score"] != float64(2) {
		t.Fatal("feedback changed score")
	}
}
