package behaviorlens

import (
	"math/rand"
	"reflect"
	"testing"
)

// bruteForce enumerates every valid path for one user and returns the best
// one by the funnel priority: most steps, then smallest duration, then
// lexicographically smallest (Timestamp, EventID) sequence.
func bruteForce(events []Event, steps []string, window int64) []int {
	n := len(events)
	var best []int
	better := func(a, b []int) bool { // is a better than b?
		if len(a) != len(b) {
			return len(a) > len(b)
		}
		da := events[a[len(a)-1]].Timestamp - events[a[0]].Timestamp
		db := events[b[len(b)-1]].Timestamp - events[b[0]].Timestamp
		if da != db {
			return da < db
		}
		for i := range a {
			ea, eb := events[a[i]], events[b[i]]
			if ea.Timestamp != eb.Timestamp {
				return ea.Timestamp < eb.Timestamp
			}
			if ea.EventID != eb.EventID {
				return ea.EventID < eb.EventID
			}
		}
		return false
	}
	for mask := 1; mask < 1<<uint(n); mask++ {
		var idx []int
		for i := 0; i < n; i++ {
			if mask&(1<<uint(i)) != 0 {
				idx = append(idx, i)
			}
		}
		if len(idx) > len(steps) {
			continue
		}
		ok := true
		for k, i := range idx {
			if events[i].Name != steps[k] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if events[idx[len(idx)-1]].Timestamp-events[idx[0]].Timestamp > window {
			continue
		}
		if best == nil || better(idx, best) {
			best = idx
		}
	}
	return best
}

// TestAgainstBruteForce generates small random streams with duplicate
// steps, events matching multiple steps, equal timestamps, and ties in
// duration among longest paths, then compares both the emitted paths and
// the per-step counts against an exhaustive reference.
func TestAgainstBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(20261002))
	names := []string{"A", "B", "C"}
	cover := map[string]int{}
	for trial := 0; trial < 3000; trial++ {
		stepCount := 1 + rng.Intn(4)
		steps := make([]string, stepCount)
		for k := range steps {
			steps[k] = names[rng.Intn(len(names))]
		}
		window := int64(rng.Intn(13))

		userCount := 1 + rng.Intn(3)
		var stream []Event
		wantPaths := map[string][]Event{}
		wantCounts := make([]int, stepCount)
		for u := 0; u < userCount; u++ {
			userID := string(rune('a' + u))
			eventCount := rng.Intn(9) // 0..8 events
			var events []Event
			for i := 0; i < eventCount; i++ {
				events = append(events, Event{
					UserID:    userID,
					Timestamp: int64(rng.Intn(13)),
					EventID:   uint64(i + 1),
					Name:      names[rng.Intn(len(names))],
				})
			}
			// Input is sorted by (timestamp, event ID) within a user.
			for i := 1; i < len(events); i++ {
				for j := i; j > 0 && events[j-1].Timestamp > events[j].Timestamp; j-- {
					events[j-1], events[j] = events[j], events[j-1]
				}
			}
			stream = append(stream, events...)

			// Track scenario coverage for this user's events.
			seen := map[string]bool{}
			for i := range events {
				for j := i + 1; j < len(events); j++ {
					if events[i].Timestamp == events[j].Timestamp {
						seen["equal timestamps"] = true
					}
				}
			}
			for k := 0; k < len(steps); k++ {
				for l := k + 1; l < len(steps); l++ {
					if steps[k] == steps[l] {
						seen["duplicate steps"] = true
					}
				}
			}
			for i := range events {
				hits := 0
				for k := 0; k < len(steps); k++ {
					if events[i].Name == steps[k] {
						hits++
					}
				}
				if hits > 1 {
					seen["event matches multiple steps"] = true
				}
			}
			// Detect ties: two distinct longest paths with equal duration.
			var longest [][]int
			for mask := 1; mask < 1<<uint(len(events)); mask++ {
				var idx []int
				for i := 0; i < len(events); i++ {
					if mask&(1<<uint(i)) != 0 {
						idx = append(idx, i)
					}
				}
				if len(idx) == 0 || len(idx) > len(steps) {
					continue
				}
				ok := true
				for k, i := range idx {
					if events[i].Name != steps[k] {
						ok = false
						break
					}
				}
				if !ok || events[idx[len(idx)-1]].Timestamp-events[idx[0]].Timestamp > window {
					continue
				}
				if len(longest) == 0 || len(idx) > len(longest[0]) {
					longest = [][]int{idx}
				} else if len(idx) == len(longest[0]) {
					longest = append(longest, idx)
				}
			}
			if len(longest) > 1 {
				d0 := events[longest[0][len(longest[0])-1]].Timestamp - events[longest[0][0]].Timestamp
				for _, p := range longest[1:] {
					if events[p[len(p)-1]].Timestamp-events[p[0]].Timestamp == d0 {
						seen["tied longest duration"] = true
					}
				}
			}
			for k := range seen {
				cover[k]++
			}

			idx := bruteForce(events, steps, window)
			if len(idx) == 0 {
				continue
			}
			path := make([]Event, len(idx))
			for i, j := range idx {
				path[i] = events[j]
			}
			wantPaths[userID] = path
			for k := 0; k < len(idx); k++ {
				wantCounts[k]++
			}
		}

		emitted, counts := collect(Build(steps, window), stream)
		if !reflect.DeepEqual(counts, wantCounts) {
			t.Fatalf("trial %d: counts = %v, want %v (steps=%v window=%d stream=%v)",
				trial, counts, wantCounts, steps, window, stream)
		}
		if !reflect.DeepEqual(emitted, wantPaths) {
			t.Fatalf("trial %d: emitted = %v, want %v (steps=%v window=%d stream=%v)",
				trial, emitted, wantPaths, steps, window, stream)
		}
	}
	for _, scenario := range []string{
		"equal timestamps",
		"duplicate steps",
		"event matches multiple steps",
		"tied longest duration",
	} {
		if cover[scenario] == 0 {
			t.Fatalf("scenario %q never exercised", scenario)
		}
		t.Logf("scenario %q exercised in %d user groups", scenario, cover[scenario])
	}
}
