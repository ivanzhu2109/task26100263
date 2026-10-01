package behaviorlens

import (
	"reflect"
	"testing"
)

// stream returns a forward-only next function over events.
func stream(events []Event) func() (Event, bool) {
	i := 0
	return func() (Event, bool) {
		if i >= len(events) {
			return Event{}, false
		}
		ev := events[i]
		i++
		return ev, true
	}
}

// collect runs Scan and gathers emitted paths per user.
func collect(q Query, events []Event) (map[uint64]Path, []int) {
	emitted := map[uint64]Path{}
	counts := Scan(q, stream(events), func(userID uint64, p Path) {
		emitted[userID] = p
	})
	return emitted, counts
}

func ev(user uint64, name string, ts int64, id uint64) Event {
	return Event{UserID: user, Name: name, Ts: ts, ID: id}
}

func pathEvents(p Path) []Event {
	return p.Events
}

// Scenario from the spec: window 10ms, events A@0, A@9, B@10, C@18,
// funnel A,B,C must find the full path starting at 9.
func TestWindowSkipsEarlyStart(t *testing.T) {
	events := []Event{
		ev(1, "A", 0, 1),
		ev(1, "A", 9, 2),
		ev(1, "B", 10, 3),
		ev(1, "C", 18, 4),
	}
	q := Build([]string{"A", "B", "C"}, 10)
	emitted, counts := collect(q, events)

	want := []Event{events[1], events[2], events[3]}
	got, ok := emitted[1]
	if !ok {
		t.Fatalf("no path emitted for user 1")
	}
	if !reflect.DeepEqual(pathEvents(got), want) {
		t.Fatalf("path = %+v, want %+v", pathEvents(got), want)
	}
	if !reflect.DeepEqual(counts, []int{1, 1, 1}) {
		t.Fatalf("counts = %v, want [1 1 1]", counts)
	}
}

// A span exactly equal to the window is valid; one millisecond over is not.
func TestWindowBoundaryInclusive(t *testing.T) {
	events := []Event{
		ev(1, "A", 5, 1),
		ev(1, "B", 15, 2), // span exactly 10: valid
		ev(2, "A", 5, 1),
		ev(2, "B", 16, 2), // span 11: over the window
	}
	q := Build([]string{"A", "B"}, 10)
	emitted, counts := collect(q, events)

	if got := pathEvents(emitted[1]); !reflect.DeepEqual(got, []Event{events[0], events[1]}) {
		t.Fatalf("user 1 path = %+v, want full A@5,B@15", got)
	}
	if got := pathEvents(emitted[2]); !reflect.DeepEqual(got, []Event{events[2]}) {
		t.Fatalf("user 2 path = %+v, want single A@5", got)
	}
	if !reflect.DeepEqual(counts, []int{2, 1}) {
		t.Fatalf("counts = %v, want [2 1]", counts)
	}
}

// Users with no event matching step 0 are not emitted and not counted.
func TestNoFirstStepNoOutput(t *testing.T) {
	events := []Event{
		ev(1, "X", 0, 1),
		ev(1, "B", 1, 2),
		ev(2, "A", 0, 1),
	}
	q := Build([]string{"A", "B"}, 100)
	emitted, counts := collect(q, events)

	if _, ok := emitted[1]; ok {
		t.Fatalf("user 1 must not be emitted")
	}
	if got := pathEvents(emitted[2]); !reflect.DeepEqual(got, []Event{events[2]}) {
		t.Fatalf("user 2 path = %+v, want single A@0", got)
	}
	if !reflect.DeepEqual(counts, []int{1, 0}) {
		t.Fatalf("counts = %v, want [1 0]", counts)
	}
}
