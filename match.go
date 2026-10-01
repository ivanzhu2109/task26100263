package behaviorlens

// selectPath finds the best path for a single user's events, which must be
// sorted by (Timestamp, EventID) ascending. Returns the chosen event indices
// in increasing order, or nil if no step matches.
//
// Priority: most steps matched, then smallest last-first timestamp delta,
// then lexicographically smallest sequence of (Timestamp, EventID) pairs.
// Because events are sorted by (Timestamp, EventID) and EventID is unique
// per user, lexicographic order of index sequences equals lexicographic
// order of (Timestamp, EventID) sequences.
func selectPath(events []Event, steps []string, window int64) []int {
	n := len(events)
	m := len(steps)
	if n == 0 || m == 0 {
		return nil
	}

	// nextMatch[k][i] = smallest index >= i matching step k, or -1.
	nextMatch := make([][]int, m)
	for k := 0; k < m; k++ {
		row := make([]int, n+1)
		row[n] = -1
		for i := n - 1; i >= 0; i-- {
			row[i] = row[i+1]
			if events[i].Name == steps[k] {
				row[i] = i
			}
		}
		nextMatch[k] = row
	}

	// earliestLastFrom greedily matches steps k..length-1 starting at or
	// after index i and returns the smallest possible index of the last
	// matched event, or -1 if those steps cannot be completed.
	earliestLastFrom := func(k, i, length int) int {
		last := -1
		for ; k < length; k++ {
			j := nextMatch[k][i]
			if j < 0 {
				return -1
			}
			last = j
			i = j + 1
		}
		return last
	}

	// minDuration returns the minimum last-first timestamp delta over all
	// paths matching the first `length` steps, or -1 if none exists.
	// Greedy-earliest completion gives the earliest possible last event
	// for each start, hence the minimum duration for that start.
	minDuration := func(length int) int64 {
		best := int64(-1)
		for i := 0; i < n; i++ {
			if events[i].Name != steps[0] {
				continue
			}
			last := i
			if length > 1 {
				last = earliestLastFrom(1, i+1, length)
				if last < 0 {
					continue
				}
			}
			dur := events[last].Timestamp - events[i].Timestamp
			if best < 0 || dur < best {
				best = dur
			}
		}
		return best
	}

	// Find the largest path length whose minimum duration fits the window.
	bestLen := 0
	bestDur := int64(-1)
	for length := m; length >= 1; length-- {
		if dur := minDuration(length); dur >= 0 && dur <= window {
			bestLen = length
			bestDur = dur
			break
		}
	}
	if bestLen == 0 {
		return nil
	}

	// Reconstruct the lexicographically smallest index sequence among
	// minimum-duration paths: at each position pick the smallest index that
	// still admits a completion within the duration budget.
	path := make([]int, 0, bestLen)
	deadline := int64(0)
	prev := 0
	for k := 0; k < bestLen; k++ {
		i := prev
		for {
			j := nextMatch[k][i]
			if j < 0 {
				return nil // unreachable: feasibility guarantees a pick
			}
			if k == 0 {
				deadline = events[j].Timestamp + bestDur
			}
			if events[j].Timestamp <= deadline {
				last := j
				if k < bestLen-1 {
					last = earliestLastFrom(k+1, j+1, bestLen)
				}
				if last >= 0 && events[last].Timestamp <= deadline {
					path = append(path, j)
					prev = j + 1
					break
				}
			}
			i = j + 1
		}
	}
	return path
}
