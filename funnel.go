// Package behaviorlens implements ordered funnel matching for BehaviorLens.
package behaviorlens

// Event is a single user event from the input stream.
type Event struct {
	UserID uint64
	Name   string
	Ts     int64  // timestamp in milliseconds
	ID     uint64 // unique within the user
}

// Query is an ordered funnel query built by Build.
type Query struct {
	steps  []string
	window int64
}

// Path is the selected sequence of events for one user.
type Path struct {
	Events []Event
}

// Build constructs a funnel query from ordered step names and a conversion
// window in milliseconds. The window is measured from the first selected
// event to the last; a difference equal to the window is still valid.
func Build(steps []string, window int64) Query {
	copied := make([]string, len(steps))
	copy(copied, steps)
	return Query{steps: copied, window: window}
}

// Scan consumes events from next (forward-only), delivers each selected path
// via emit, and returns the cumulative reached-user count per step: a user
// whose selected path has length m contributes one to each of the first m
// steps. Users with no path are not emitted and not counted.
//
// Events must arrive grouped by user, ordered by (UserID, Ts, ID).
func Scan(query Query, next func() (Event, bool), emit func(userID uint64, path Path)) []int {
	counts := make([]int, len(query.steps))
	if len(query.steps) == 0 {
		return counts
	}

	var buf []Event
	var curUser uint64
	haveUser := false

	flush := func() {
		if !haveUser {
			return
		}
		best := selectPath(query, buf)
		if len(best) > 0 {
			emit(curUser, Path{Events: best})
			for i := 0; i < len(best); i++ {
				counts[i]++
			}
		}
		buf = buf[:0]
	}

	for {
		ev, ok := next()
		if !ok {
			break
		}
		if !haveUser || ev.UserID != curUser {
			flush()
			curUser = ev.UserID
			haveUser = true
		}
		buf = append(buf, ev)
	}
	flush()
	return counts
}

// selectPath picks the single best valid path for one user's events, which
// must be ordered by (Ts, ID). It returns nil when no event matches step 0.
//
// dp[k][i] is the best path of length k+1 ending exactly at events[i]
// (nil when events[i] cannot end such a path). Extending a fixed set of
// candidates with the same event preserves their order under better(), so
// keeping only the best path per (step, ending event) is exact.
func selectPath(query Query, events []Event) []Event {
	m := len(query.steps)
	n := len(events)

	dp := make([][][]Event, m)
	for k := range dp {
		dp[k] = make([][]Event, n)
	}

	var best []Event
	consider := func(cand []Event) {
		if cand != nil && (best == nil || better(cand, best)) {
			best = cand
		}
	}

	for k := 0; k < m; k++ {
		for i := 0; i < n; i++ {
			if events[i].Name != query.steps[k] {
				continue
			}
			if k == 0 {
				dp[0][i] = []Event{events[i]}
				consider(dp[0][i])
				continue
			}
			for j := 0; j < i; j++ {
				prev := dp[k-1][j]
				if prev == nil {
					continue
				}
				if events[i].Ts-prev[0].Ts > query.window {
					continue
				}
				cand := make([]Event, len(prev)+1)
				copy(cand, prev)
				cand[len(prev)] = events[i]
				if dp[k][i] == nil || better(cand, dp[k][i]) {
					dp[k][i] = cand
				}
			}
			consider(dp[k][i])
		}
	}
	return best
}
