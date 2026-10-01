// Package behaviorlens implements ordered funnel matching over a
// forward-only event stream.
package behaviorlens

// Event is a single user event in the input stream.
type Event struct {
	UserID    string
	Timestamp int64  // milliseconds
	EventID   uint64 // unique within a user
	Name      string
}

// Query is a built funnel query: ordered steps plus a conversion window.
type Query struct {
	steps  []string
	window int64
}

// Build constructs a funnel query from ordered step event names and a
// conversion window in milliseconds. The window is measured from the
// actually selected first-step event to the last-step event; a delta equal
// to the window is still valid.
func Build(steps []string, window int64) Query {
	copied := make([]string, len(steps))
	copy(copied, steps)
	return Query{steps: copied, window: window}
}

// Scan reads events from next (forward-only, ordered by user, timestamp,
// event ID, with each user's events contiguous), delivers each matched
// user's chosen path via emit, and returns per-step cumulative reached-user
// counts. Users whose events never match the first step are not emitted and
// contribute nothing.
func Scan(query Query, next func() (Event, bool), emit func(userID string, path []Event)) []int {
	counts := make([]int, len(query.steps))
	var buf []Event
	flush := func() {
		if len(buf) == 0 {
			return
		}
		idx := selectPath(buf, query.steps, query.window)
		if len(idx) > 0 {
			path := make([]Event, len(idx))
			for i, j := range idx {
				path[i] = buf[j]
			}
			emit(buf[0].UserID, path)
			for k := 0; k < len(idx); k++ {
				counts[k]++
			}
		}
		buf = buf[:0]
	}
	for {
		ev, ok := next()
		if !ok {
			break
		}
		if len(buf) > 0 && ev.UserID != buf[0].UserID {
			flush()
		}
		buf = append(buf, ev)
	}
	flush()
	return counts
}
