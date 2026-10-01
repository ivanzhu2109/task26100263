package behaviorlens

import (
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

// enumeratePaths lists every valid path for one user's events: for each
// length m, every strictly increasing event subsequence matching the first m
// steps whose last-first time span fits the window.
func enumeratePaths(events []Event, steps []string, window int64) [][]Event {
	var paths [][]Event
	var cur []Event
	var walk func(k, from int)
	walk = func(k, from int) {
		if k == len(steps) {
			return
		}
		for i := from; i < len(events); i++ {
			if events[i].Name != steps[k] {
				continue
			}
			if len(cur) > 0 && events[i].Ts-cur[0].Ts > window {
				continue
			}
			cur = append(cur, events[i])
			paths = append(paths, append([]Event(nil), cur...))
			walk(k+1, i+1)
			cur = cur[:len(cur)-1]
		}
	}
	walk(0, 0)
	return paths
}

// referenceBest picks the unique best path by exhaustive enumeration.
func referenceBest(events []Event, steps []string, window int64) []Event {
	paths := enumeratePaths(events, steps, window)
	var best []Event
	for _, p := range paths {
		if best == nil || better(p, best) {
			best = p
		}
	}
	// The selection rules must yield a unique winner: exactly one path
	// (the best itself) is incomparable with best.
	equivalent := 0
	for _, p := range paths {
		if !better(p, best) && !better(best, p) {
			equivalent++
		}
	}
	if best != nil && equivalent != 1 {
		panic("reference selection is not unique")
	}
	return best
}

func TestRandomAgainstExhaustive(t *testing.T) {
	rng := rand.New(rand.NewSource(20261002))
	names := []string{"A", "B"}

	for trial := 0; trial < 3000; trial++ {
		numSteps := 1 + rng.Intn(4)
		steps := make([]string, numSteps)
		for i := range steps {
			steps[i] = names[rng.Intn(len(names))] // duplicate steps likely
		}
		window := int64(rng.Intn(7))

		numUsers := 1 + rng.Intn(4)
		var events []Event
		userEvents := map[uint64][]Event{}
		for u := 1; u <= numUsers; u++ {
			uid := uint64(u)
			n := rng.Intn(11)
			var evs []Event
			for i := 0; i < n; i++ {
				evs = append(evs, Event{
					UserID: uid,
					Name:   names[rng.Intn(len(names))],
					Ts:     int64(rng.Intn(8)), // equal timestamps likely
					ID:     uint64(i + 1),
				})
			}
			sort.Slice(evs, func(a, b int) bool {
				if evs[a].Ts != evs[b].Ts {
					return evs[a].Ts < evs[b].Ts
				}
				return evs[a].ID < evs[b].ID
			})
			// Reassign IDs in sorted order so (Ts, ID) order matches input.
			for i := range evs {
				evs[i].ID = uint64(i + 1)
			}
			userEvents[uid] = evs
			events = append(events, evs...)
		}

		q := Build(steps, window)
		emitted, counts := collect(q, events)

		wantCounts := make([]int, numSteps)
		for u := 1; u <= numUsers; u++ {
			uid := uint64(u)
			want := referenceBest(userEvents[uid], steps, window)
			got, ok := emitted[uid]
			if len(want) == 0 {
				if ok {
					t.Fatalf("trial %d user %d: unexpected emit %+v", trial, uid, got.Events)
				}
				continue
			}
			if !ok {
				t.Fatalf("trial %d user %d: missing emit, want %+v (steps=%v window=%d events=%+v)",
					trial, uid, want, steps, window, userEvents[uid])
			}
			if !reflect.DeepEqual(got.Events, want) {
				t.Fatalf("trial %d user %d: path %+v, want %+v (steps=%v window=%d events=%+v)",
					trial, uid, got.Events, want, steps, window, userEvents[uid])
			}
			for i := 0; i < len(want); i++ {
				wantCounts[i]++
			}
		}
		if !reflect.DeepEqual(counts, wantCounts) {
			t.Fatalf("trial %d: counts %v, want %v (steps=%v window=%d)",
				trial, counts, wantCounts, steps, window)
		}
	}
}
