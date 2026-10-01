package behaviorlens

import (
	"reflect"
	"testing"
)

// collect runs Scan over events and returns emitted paths keyed by user
// plus the per-step counts.
func collect(q Query, events []Event) (map[string][]Event, []int) {
	i := 0
	next := func() (Event, bool) {
		if i >= len(events) {
			return Event{}, false
		}
		ev := events[i]
		i++
		return ev, true
	}
	emitted := map[string][]Event{}
	emit := func(userID string, path []Event) {
		emitted[userID] = path
	}
	counts := Scan(q, next, emit)
	return emitted, counts
}

func names(path []Event) []string {
	out := make([]string, len(path))
	for i, ev := range path {
		out[i] = ev.Name
	}
	return out
}

func timestamps(path []Event) []int64 {
	out := make([]int64, len(path))
	for i, ev := range path {
		out[i] = ev.Timestamp
	}
	return out
}

// Window 10ms; events A@0, A@9, B@10, C@18; funnel A,B,C must find the
// full path starting at 9 (A@0 cannot reach C within the window).
func TestScanFindsPathStartingAtNine(t *testing.T) {
	events := []Event{
		{UserID: "u1", Timestamp: 0, EventID: 1, Name: "A"},
		{UserID: "u1", Timestamp: 9, EventID: 2, Name: "A"},
		{UserID: "u1", Timestamp: 10, EventID: 3, Name: "B"},
		{UserID: "u1", Timestamp: 18, EventID: 4, Name: "C"},
	}
	emitted, counts := collect(Build([]string{"A", "B", "C"}, 10), events)
	path, ok := emitted["u1"]
	if !ok {
		t.Fatal("expected a path for u1")
	}
	if got, want := names(path), []string{"A", "B", "C"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("path names = %v, want %v", got, want)
	}
	if got, want := timestamps(path), []int64{9, 10, 18}; !reflect.DeepEqual(got, want) {
		t.Fatalf("path timestamps = %v, want %v", got, want)
	}
	if got, want := counts, []int{1, 1, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("counts = %v, want %v", got, want)
	}
}

// A delta exactly equal to the window is still valid.
func TestWindowBoundaryInclusive(t *testing.T) {
	events := []Event{
		{UserID: "u1", Timestamp: 0, EventID: 1, Name: "A"},
		{UserID: "u1", Timestamp: 10, EventID: 2, Name: "B"},
	}
	emitted, counts := collect(Build([]string{"A", "B"}, 10), events)
	path, ok := emitted["u1"]
	if !ok {
		t.Fatal("expected a path for u1 at the exact window boundary")
	}
	if got, want := timestamps(path), []int64{0, 10}; !reflect.DeepEqual(got, want) {
		t.Fatalf("path timestamps = %v, want %v", got, want)
	}
	if got, want := counts, []int{1, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("counts = %v, want %v", got, want)
	}

	// One millisecond beyond the window must fail the second step.
	emitted, counts = collect(Build([]string{"A", "B"}, 9), events)
	if _, ok := emitted["u1"]; !ok {
		t.Fatal("expected u1 to still emit its single-step path")
	}
	if got, want := counts, []int{1, 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("counts = %v, want %v", got, want)
	}
}

// Users that never match the first step emit nothing and count nothing.
func TestNoFirstStepMatch(t *testing.T) {
	events := []Event{
		{UserID: "u1", Timestamp: 0, EventID: 1, Name: "X"},
		{UserID: "u2", Timestamp: 1, EventID: 1, Name: "A"},
	}
	emitted, counts := collect(Build([]string{"A", "B"}, 10), events)
	if _, ok := emitted["u1"]; ok {
		t.Fatal("u1 must not be emitted")
	}
	if _, ok := emitted["u2"]; !ok {
		t.Fatal("u2 must be emitted")
	}
	if got, want := counts, []int{1, 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("counts = %v, want %v", got, want)
	}
}
