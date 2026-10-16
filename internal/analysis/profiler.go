package analysis

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

type Metrics struct {
	Calls          int   `json:"calls"`
	Waiting        int   `json:"waiting"`
	Running        int   `json:"running"`
	Refused        int   `json:"refused"`
	Errors         int   `json:"errors"`
	Interrupted    int   `json:"interrupted"`
	PayloadBytes   int64 `json:"payloadBytes"`
	LatencySamples int   `json:"latencySamples"`
	P50            int64 `json:"p50"`
	P95            int64 `json:"p95"`
}

func Duration(r Row) *int64 {
	if r.Kind != "tool" || (r.Status != recorder.StatusOK && r.Status != recorder.StatusError) || r.Request != nil && r.Request.Decision == "approve" {
		return nil
	}
	return r.DurationMs
}
func Measure(rows []Row) Metrics {
	m := Metrics{}
	durations := []int64{}
	for _, r := range rows {
		if r.Kind != "tool" {
			continue
		}
		m.Calls++
		switch r.Status {
		case recorder.StatusAwaiting:
			m.Waiting++
		case recorder.StatusPending:
			m.Running++
		case recorder.StatusBlocked, recorder.StatusRejected:
			m.Refused++
		case recorder.StatusError:
			m.Errors++
		case recorder.StatusInterrupted:
			m.Interrupted++
		}
		if r.RequestBytes != nil {
			m.PayloadBytes += int64(*r.RequestBytes)
		}
		if r.ResponseBytes != nil {
			m.PayloadBytes += int64(*r.ResponseBytes)
		}
		if d := Duration(r); d != nil {
			durations = append(durations, *d)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	m.LatencySamples = len(durations)
	if len(durations) > 0 {
		m.P50 = durations[int(math.Ceil(float64(len(durations))*.5))-1]
		m.P95 = durations[int(math.Ceil(float64(len(durations))*.95))-1]
	}
	return m
}

type ToolActivity struct {
	Name    string  `json:"name"`
	Metrics Metrics `json:"metrics"`
}
type Profile struct {
	Calls   int            `json:"calls"`
	Tools   []ToolActivity `json:"tools"`
	Longest []Row          `json:"longest"`
	Largest []Row          `json:"largest"`
}

func ToolProfile(rows []Row) Profile {
	p := Profile{Tools: []ToolActivity{}, Longest: []Row{}, Largest: []Row{}}
	groups := map[string][]Row{}
	for _, r := range rows {
		if r.Kind != "tool" {
			continue
		}
		p.Calls++
		groups[r.Title] = append(groups[r.Title], r)
		if Duration(r) != nil {
			p.Longest = append(p.Longest, r)
		}
		if r.ResponseBytes != nil {
			p.Largest = append(p.Largest, r)
		}
	}
	for name, entries := range groups {
		p.Tools = append(p.Tools, ToolActivity{name, Measure(entries)})
	}
	sort.Slice(p.Tools, func(i, j int) bool {
		a, b := p.Tools[i], p.Tools[j]
		if a.Metrics.Calls != b.Metrics.Calls {
			return a.Metrics.Calls > b.Metrics.Calls
		}
		return a.Name < b.Name
	})
	Sort(p.Longest, "duration")
	Sort(p.Largest, "response")
	if len(p.Longest) > 5 {
		p.Longest = p.Longest[:5]
	}
	if len(p.Largest) > 5 {
		p.Largest = p.Largest[:5]
	}
	for i := range p.Longest {
		p.Longest[i] = Summary(p.Longest[i])
	}
	for i := range p.Largest {
		p.Largest[i] = Summary(p.Largest[i])
	}
	return p
}

type Range struct {
	StartMs int64 `json:"startMs"`
	EndMs   int64 `json:"endMs"`
}
type Timeline struct {
	WindowStartMs   *int64  `json:"windowStartMs,omitempty"`
	WindowEndMs     *int64  `json:"windowEndMs,omitempty"`
	StartMs         *int64  `json:"startMs,omitempty"`
	EndMs           *int64  `json:"endMs,omitempty"`
	SpanMs          int64   `json:"spanMs"`
	CoveredMs       int64   `json:"coveredMs"`
	UnattributedMs  int64   `json:"unattributedMs"`
	OverlapMs       int64   `json:"overlapMs"`
	PeakConcurrency int     `json:"peakConcurrency"`
	IncompleteCalls int     `json:"incompleteCalls"`
	GapCount        int     `json:"gapCount"`
	Gaps            []Range `json:"gaps"`
}

// PageWaterfall keeps scope totals while zooming the plotted bounds and gaps
// to the current page. A late page must not inherit the first 200 global gaps.
func PageWaterfall(session recorder.Session, rows, page []Row, scoped bool) Timeline {
	t := Waterfall(session, rows, scoped)
	for _, r := range page {
		if r.Kind != "tool" || r.Span == nil {
			continue
		}
		start, end := r.Span.StartMs, r.Span.StartMs
		if r.Span.EndMs != nil {
			end = *r.Span.EndMs
		}
		if t.WindowStartMs == nil || start < *t.WindowStartMs {
			v := start
			t.WindowStartMs = &v
		}
		if t.WindowEndMs == nil || end > *t.WindowEndMs {
			v := end
			t.WindowEndMs = &v
		}
	}
	if t.WindowStartMs == nil {
		return t
	}
	clipped := []Row{}
	for _, r := range rows {
		if r.Kind != "tool" || r.Span == nil || r.Span.EndMs == nil {
			continue
		}
		start, end := max(*t.WindowStartMs, r.Span.StartMs), min(*t.WindowEndMs, *r.Span.EndMs)
		if end < start {
			continue
		}
		copy := r
		copy.Request, copy.Response, copy.Resolution, copy.Event = nil, nil, nil, nil
		copy.Span = &Span{StartMs: start, EndMs: &end}
		clipped = append(clipped, copy)
	}
	window := Waterfall(recorder.Session{StartedAt: time.UnixMilli(*t.WindowStartMs), EndedAt: time.UnixMilli(*t.WindowEndMs)}, clipped, false)
	t.Gaps, t.GapCount = window.Gaps, window.GapCount
	return t
}

// Waterfall anchors its end to the last observed evidence, never time.Now.
// It unions matched intervals, so concurrency cannot inflate covered time.
// Pending, ambiguous, synthetic and invalid-ended calls are points only.
func Waterfall(session recorder.Session, rows []Row, scoped bool) Timeline {
	t := Timeline{Gaps: []Range{}}
	ranges := []Range{}
	var sum int64
	observe := func(stamp int64) {
		if t.StartMs == nil || stamp < *t.StartMs {
			v := stamp
			t.StartMs = &v
		}
		if t.EndMs == nil || stamp > *t.EndMs {
			v := stamp
			t.EndMs = &v
		}
	}
	if !scoped && !session.StartedAt.IsZero() {
		observe(session.StartedAt.UnixMilli())
	}
	if !scoped && !session.EndedAt.IsZero() {
		observe(session.EndedAt.UnixMilli())
	}
	for _, r := range rows {
		for _, e := range []*recorder.Event{r.Request, r.Response, r.Resolution, r.Event} {
			if e != nil && !e.Timestamp.IsZero() {
				observe(e.Timestamp.UnixMilli())
			}
		}
		if r.Kind != "tool" {
			continue
		}
		if r.Span == nil || r.Span.EndMs == nil {
			t.IncompleteCalls++
			continue
		}
		span := Range{r.Span.StartMs, *r.Span.EndMs}
		ranges = append(ranges, span)
		sum += span.EndMs - span.StartMs
	}
	if t.StartMs == nil {
		return t
	}
	t.SpanMs = *t.EndMs - *t.StartMs
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].StartMs < ranges[j].StartMs })
	type edge struct {
		at    int64
		delta int
	}
	edges := []edge{}
	cursor := *t.StartMs
	gap := func(a, b int64) {
		if b <= a {
			return
		}
		t.GapCount++
		if len(t.Gaps) < 200 {
			t.Gaps = append(t.Gaps, Range{a, b})
		}
	}
	for i := 0; i < len(ranges); {
		span := ranges[i]
		if span.EndMs > span.StartMs {
			edges = append(edges, edge{span.StartMs, 1}, edge{span.EndMs, -1})
		}
		j := i + 1
		for j < len(ranges) && ranges[j].StartMs <= span.EndMs {
			r := ranges[j]
			if r.EndMs > r.StartMs {
				edges = append(edges, edge{r.StartMs, 1}, edge{r.EndMs, -1})
			}
			if r.EndMs > span.EndMs {
				span.EndMs = r.EndMs
			}
			j++
		}
		gap(cursor, span.StartMs)
		t.CoveredMs += span.EndMs - span.StartMs
		if span.EndMs > cursor {
			cursor = span.EndMs
		}
		i = j
	}
	gap(cursor, *t.EndMs)
	t.UnattributedMs = t.SpanMs - t.CoveredMs
	t.OverlapMs = sum - t.CoveredMs
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].at != edges[j].at {
			return edges[i].at < edges[j].at
		}
		return edges[i].delta < edges[j].delta
	})
	count := 0
	for _, e := range edges {
		count += e.delta
		if count > t.PeakConcurrency {
			t.PeakConcurrency = count
		}
	}
	return t
}

type ScopeOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func Scopes(rows []Row) []ScopeOption {
	options := []ScopeOption{}
	seen := map[string]bool{}
	for _, r := range rows {
		id := Scope(r)
		if seen[id] {
			continue
		}
		seen[id] = true
		e := Evidence(r)
		label := "Recording / unidentified session"
		if e != nil {
			source := e.Source
			switch source {
			case "cursor-hook":
				source = "Cursor"
			case "claude-hook":
				source = "Claude Code"
			case "mcp-stdio":
				source = "MCP"
			}
			if e.AgentSessionID != "" {
				label = source + " · " + e.AgentSessionID
			} else if e.ConnectionID != "" {
				label = "MCP connection · " + e.ConnectionID
			}
		}
		options = append(options, ScopeOption{id, label})
	}
	return options
}
func InScope(rows []Row, scope string) []Row {
	if scope == "" {
		return rows
	}
	out := []Row{}
	for _, r := range rows {
		if Scope(r) == scope {
			out = append(out, r)
		}
	}
	return out
}
func Filter(rows []Row, view, filter, tool, query string) []Row {
	out := []Row{}
	q := strings.ToLower(strings.TrimSpace(query))
	for _, r := range rows {
		if view != "events" {
			if r.Kind != "tool" {
				continue
			}
			if tool != "" && r.Title != tool {
				continue
			}
			switch filter {
			case "errors":
				if r.Status != recorder.StatusError {
					continue
				}
			case "open":
				if r.Status != recorder.StatusPending {
					continue
				}
			case "interrupted":
				if r.Status != recorder.StatusInterrupted {
					continue
				}
			case "unresolved":
				if r.Status != "unknown" {
					continue
				}
			}
		}
		if q != "" {
			text := r.Title + " " + r.Subtitle
			if view != "events" && r.Request != nil {
				text += " " + string(r.Request.Payload)
			}
			if !strings.Contains(strings.ToLower(text), q) {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}
func Sort(rows []Row, order string) {
	if order == "time" || order == "" {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		var a, b *int64
		if order == "duration" {
			a, b = Duration(rows[i]), Duration(rows[j])
		} else {
			if rows[i].ResponseBytes != nil {
				v := int64(*rows[i].ResponseBytes)
				a = &v
			}
			if rows[j].ResponseBytes != nil {
				v := int64(*rows[j].ResponseBytes)
				b = &v
			}
		}
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return *a > *b
	})
}
