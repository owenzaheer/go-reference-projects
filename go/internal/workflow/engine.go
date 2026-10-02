package workflow

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Problem struct {
	Status  int
	Message string
}

func (p Problem) Error() string { return p.Message }

type Record map[string]any
type Engine struct {
	mu           sync.Mutex
	queue        chan Record
	records      map[string]Record
	appointments map[string]Record
	grades       map[string]Record
	audit        []Record
}

func New() *Engine {
	return &Engine{queue: make(chan Record, 4), records: map[string]Record{}, appointments: map[string]Record{}, grades: map[string]Record{}, audit: []Record{}}
}
func text(p Record, k string) (string, error) {
	v, ok := p[k].(string)
	if !ok || len(v) < 1 || len(v) > 100 {
		return "", Problem{422, k + " is required"}
	}
	return v, nil
}
func num(p Record, k string, min float64) (float64, error) {
	var n float64
	switch v := p[k].(type) {
	case float64:
		n = v
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	default:
		return 0, Problem{422, k + " must be an integer"}
	}
	if n < min || n != float64(int64(n)) {
		return 0, Problem{422, k + " must be a nonnegative integer"}
	}
	return n, nil
}
func clone(p Record) Record { b, _ := json.Marshal(p); var r Record; json.Unmarshal(b, &r); return r }
func rows(table map[string]Record) []Record {
	r := []Record{}
	for _, v := range table {
		r = append(r, clone(v))
	}
	return r
}
func (e *Engine) State() Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Record{"queueDepth": len(e.queue), "events": rows(e.records), "appointments": rows(e.appointments), "grades": rows(e.grades), "audit": append([]Record{}, e.audit...)}
}
func (e *Engine) Command(action string, p Record, role string) (Record, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, err := e.execute(action, p, role)
	if err == nil {
		e.audit = append(e.audit, Record{"action": action, "role": role, "payload": clone(p)})
	}
	return clone(r), err
}
func (e *Engine) execute(action string, p Record, role string) (Record, error) {
	if action == "enqueue" {
		id, err := text(p, "id")
		if err != nil {
			return nil, err
		}
		kind, err := text(p, "kind")
		if err != nil {
			return nil, err
		}
		qty, err := num(p, "quantity", 1)
		if err != nil {
			return nil, err
		}
		if kind != "order" {
			return nil, Problem{422, "Only order events are supported"}
		}
		encoded, _ := json.Marshal(p)
		if old, ok := e.records[id]; ok {
			if old["body"] != string(encoded) {
				return nil, Problem{409, "Event id reused with different payload"}
			}
			return old, nil
		}
		r := Record{"id": id, "quantity": qty, "body": string(encoded), "status": "queued"}
		select {
		case e.queue <- r:
			e.records[id] = r
			return r, nil
		default:
			return nil, Problem{429, "Queue is full; retry without changing the event id"}
		}
	}
	if action == "drain" {
		if role != "operator" {
			return nil, Problem{403, "Operator required"}
		}
		batch := []Record{}
		for len(e.queue) > 0 {
			batch = append(batch, <-e.queue)
		}
		jobs := make(chan Record, len(batch))
		results := make(chan Record, len(batch))
		var workers sync.WaitGroup
		for i := 0; i < 2; i++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				for r := range jobs {
					copy := clone(r)
					if fail, ok := p["simulateFailure"].(bool); ok && fail {
						copy["status"] = "retry"
					} else {
						copy["status"] = "processed"
					}
					results <- copy
				}
			}()
		}
		for _, r := range batch {
			jobs <- r
		}
		close(jobs)
		workers.Wait()
		close(results)
		done := []Record{}
		for r := range results {
			e.records[r["id"].(string)] = r
			done = append(done, r)
			if r["status"] == "retry" {
				e.queue <- r
			}
		}
		return Record{"results": done, "workers": 2}, nil
	}
	if action == "book" {
		id, err := text(p, "id")
		if err != nil {
			return nil, err
		}
		tenant, err := text(p, "tenant")
		if err != nil {
			return nil, err
		}
		resource, err := text(p, "resource")
		if err != nil {
			return nil, err
		}
		start, err := num(p, "start", 1)
		if err != nil {
			return nil, err
		}
		end, err := num(p, "end", 1)
		if err != nil {
			return nil, err
		}
		if role != tenant {
			return nil, Problem{403, "Token tenant does not match appointment tenant"}
		}
		if start < float64(time.Now().Unix()) || end <= start || end-start > 86400 {
			return nil, Problem{422, "Future interval of at most one day required"}
		}
		if old, ok := e.appointments[id]; ok {
			if old["tenant"] != tenant || old["resource"] != resource || old["start"] != start || old["end"] != end {
				return nil, Problem{409, "Appointment id reused with different payload"}
			}
			return old, nil
		}
		for _, r := range e.appointments {
			if r["status"] == "booked" && r["tenant"] == tenant && r["resource"] == resource && start < r["end"].(float64) && end > r["start"].(float64) {
				return nil, Problem{409, "Appointment overlaps an existing booking"}
			}
		}
		r := Record{"id": id, "tenant": tenant, "resource": resource, "start": start, "end": end, "status": "booked"}
		e.appointments[id] = r
		return r, nil
	}
	if action == "cancel" {
		id, err := text(p, "id")
		if err != nil {
			return nil, err
		}
		r, ok := e.appointments[id]
		if !ok {
			return nil, Problem{404, "Appointment not found"}
		}
		if r["tenant"] != role {
			return nil, Problem{403, "Cross-tenant cancellation rejected"}
		}
		r["status"] = "cancelled"
		return r, nil
	}
	if action == "submit" {
		if role != "learner" && role != "instructor" {
			return nil, Problem{403, "Learner role required"}
		}
		id, err := text(p, "id")
		if err != nil {
			return nil, err
		}
		student, err := text(p, "student")
		if err != nil {
			return nil, err
		}
		deadline, err := num(p, "deadline", 0)
		if err != nil || deadline < float64(time.Now().Unix()) {
			return nil, Problem{422, "Submission deadline has passed"}
		}
		answers, ok := p["answers"].([]any)
		if !ok || len(answers) != 3 {
			return nil, Problem{422, "Three answer strings required"}
		}
		score := 0
		expected := []string{"transaction", "idempotency", "audit"}
		for i, a := range answers {
			answer, ok := a.(string)
			if !ok {
				return nil, Problem{422, "Answer must be a string"}
			}
			if answer == expected[i] {
				score++
			}
		}
		if _, exists := e.grades[id]; exists {
			return nil, Problem{409, "Duplicate assessment event"}
		}
		r := Record{"id": id, "student": student, "score": score, "status": "pending", "feedback": "", "version": float64(0)}
		e.grades[id] = r
		return r, nil
	}
	if action == "feedback" || action == "publish" {
		if role != "instructor" {
			return nil, Problem{403, "Instructor required"}
		}
		id, err := text(p, "id")
		if err != nil {
			return nil, err
		}
		r, ok := e.grades[id]
		if !ok {
			return nil, Problem{404, "Submission not found"}
		}
		version, err := num(p, "version", 0)
		if err != nil {
			return nil, err
		}
		if version != r["version"] {
			return nil, Problem{409, "Stale assessment version"}
		}
		if action == "feedback" {
			feedback, err := text(p, "feedback")
			if err != nil {
				return nil, err
			}
			r["feedback"] = feedback
			r["status"] = "review"
		} else {
			if r["status"] != "review" {
				return nil, Problem{409, "Instructor feedback review required"}
			}
			r["status"] = "published"
		}
		r["version"] = version + 1
		return r, nil
	}
	return nil, Problem{404, fmt.Sprintf("Unknown action %q", action)}
}
