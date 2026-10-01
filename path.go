package behaviorlens

// span returns the time difference between the last and first event of a
// non-empty path. Single-event paths have zero span.
func span(p []Event) int64 {
	return p[len(p)-1].Ts - p[0].Ts
}

// better reports whether path a is strictly preferred over path b under the
// selection rules: more steps, then smaller time span, then lexicographically
// smaller (Ts, ID) tuple sequence. Paths must be non-empty.
func better(a, b []Event) bool {
	if len(a) != len(b) {
		return len(a) > len(b)
	}
	sa, sb := span(a), span(b)
	if sa != sb {
		return sa < sb
	}
	for i := range a {
		if a[i].Ts != b[i].Ts {
			return a[i].Ts < b[i].Ts
		}
		if a[i].ID != b[i].ID {
			return a[i].ID < b[i].ID
		}
	}
	return false
}
