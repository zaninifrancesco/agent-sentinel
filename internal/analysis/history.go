package analysis

import (
	"math"
	"sort"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

type Recording struct {
	Session  recorder.Session
	Revision uint64
	Rows     []Row
	Live     bool
}

type HistoryQuery struct {
	From, To            time.Time
	Agent, Project, Day string
}

type UsageMetrics struct {
	Calls          int      `json:"calls"`
	OK             int      `json:"ok"`
	Errors         int      `json:"errors"`
	Open           int      `json:"open"`
	Interrupted    int      `json:"interrupted"`
	Other          int      `json:"other"`
	ErrorSamples   int      `json:"errorSamples"`
	ErrorRate      *float64 `json:"errorRate"`
	LatencySamples int      `json:"latencySamples"`
	P50            *int64   `json:"p50"`
	P95            *int64   `json:"p95"`
	ToolDurationMs int64    `json:"toolDurationMs"`
	RetainedBytes  int64    `json:"retainedBytes"`
}

type Breakdown struct {
	Name string `json:"name"`
	UsageMetrics
}

type HistorySession struct {
	RecordingID     string    `json:"recordingId"`
	Scope           string    `json:"scope"`
	Agent           string    `json:"agent"`
	NativeSessionID string    `json:"nativeSessionId,omitempty"`
	Projects        []string  `json:"projects"`
	First           time.Time `json:"first"`
	Last            time.Time `json:"last"`
	Live            bool      `json:"live"`
	Ended           bool      `json:"ended"`
	RecoveredTail   bool      `json:"recoveredTail"`
	UsageMetrics
}

type History struct {
	Version           string           `json:"version"`
	From              time.Time        `json:"from"`
	To                time.Time        `json:"to"`
	Timezone          string           `json:"timezone"`
	Totals            UsageMetrics     `json:"totals"`
	Previous          UsageMetrics     `json:"previous"`
	RecordingCount    int              `json:"recordingCount"`
	NativeSessions    int              `json:"nativeSessions"`
	Days              []Breakdown      `json:"days"`
	Agents            []Breakdown      `json:"agents"`
	Projects          []Breakdown      `json:"projects"`
	Tools             []Breakdown      `json:"tools"`
	AvailableAgents   []string         `json:"availableAgents"`
	AvailableProjects []string         `json:"availableProjects"`
	Sessions          []HistorySession `json:"sessions"`
	UndatedCalls      int              `json:"undatedCalls"`
}

func usage(rows []Row) UsageMetrics {
	m := UsageMetrics{}
	durations := []int64{}
	for _, row := range rows {
		m.Calls++
		switch row.Status {
		case recorder.StatusOK:
			m.OK++
		case recorder.StatusError:
			m.Errors++
		case recorder.StatusPending, recorder.StatusAwaiting:
			m.Open++
		case recorder.StatusInterrupted:
			m.Interrupted++
		default:
			m.Other++
		}
		if row.RequestBytes != nil {
			m.RetainedBytes += int64(*row.RequestBytes)
		}
		if row.ResponseBytes != nil {
			m.RetainedBytes += int64(*row.ResponseBytes)
		}
		if d := Duration(row); d != nil {
			durations = append(durations, *d)
			m.ToolDurationMs += *d
		}
	}
	m.ErrorSamples = m.OK + m.Errors
	if m.ErrorSamples > 0 {
		rate := float64(m.Errors) / float64(m.ErrorSamples)
		m.ErrorRate = &rate
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	m.LatencySamples = len(durations)
	if len(durations) > 0 {
		p50, p95 := durations[int(math.Ceil(float64(len(durations))*.5))-1], durations[int(math.Ceil(float64(len(durations))*.95))-1]
		m.P50, m.P95 = &p50, &p95
	}
	return m
}

func dimension(row Row) (agent, project string) {
	agent, project = "unknown", "unknown"
	if e := Evidence(row); e != nil {
		if e.Source != "" {
			agent = e.Source
		}
		if e.ProjectRef != "" {
			project = e.ProjectRef
		}
	}
	return
}

func breakdown(groups map[string][]Row) []Breakdown {
	result := []Breakdown{}
	for name, rows := range groups {
		result = append(result, Breakdown{name, usage(rows)})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Calls != result[j].Calls {
			return result[i].Calls > result[j].Calls
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// Historical uses request/observation time in UTC, [from,to). Missing archive
// coverage is never inferred from an empty bucket. Percentiles use all samples.
func Historical(recordings []Recording, q HistoryQuery) History {
	h := History{Version: "analytics-v1", From: q.From, To: q.To, Timezone: "UTC", Days: []Breakdown{}, Sessions: []HistorySession{}}
	current, previous := []Row{}, []Row{}
	agents, projects, tools, days := map[string][]Row{}, map[string][]Row{}, map[string][]Row{}, map[string][]Row{}
	availableAgents, availableProjects, recordingIDs, nativeIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	previousFrom := q.From.Add(-q.To.Sub(q.From))
	for _, recording := range recordings {
		groups := map[string][]Row{}
		hasTool := false
		for _, row := range recording.Rows {
			if row.Kind != "tool" {
				continue
			}
			hasTool = true
			agent, project := dimension(row)
			availableAgents[agent], availableProjects[project] = true, true
			if q.Agent != "" && q.Agent != agent || q.Project != "" && q.Project != project {
				continue
			}
			if row.StartedAt.IsZero() {
				h.UndatedCalls++
				continue
			}
			if row.StartedAt.Before(q.From) {
				if !row.StartedAt.Before(previousFrom) {
					previous = append(previous, row)
				}
				continue
			}
			if !row.StartedAt.Before(q.To) {
				continue
			}
			day := row.StartedAt.UTC().Format("2006-01-02")
			// Day selects the drill-down list; period totals and trends stay fixed.
			if q.Day == "" || q.Day == day {
				groups[Scope(row)] = append(groups[Scope(row)], row)
			}
			current = append(current, row)
			agents[agent], projects[project], tools[row.Title], days[day] = append(agents[agent], row), append(projects[project], row), append(tools[row.Title], row), append(days[day], row)
			recordingIDs[recording.Session.ID] = true
			if e := Evidence(row); e != nil && e.AgentSessionID != "" {
				nativeIDs[recording.Session.ID+"/"+Scope(row)] = true
			}
		}
		if !hasTool && (q.Agent == "" || q.Agent == "unknown") && (q.Project == "" || q.Project == "unknown") && !recording.Session.StartedAt.Before(q.From) && recording.Session.StartedAt.Before(q.To) {
			recordingIDs[recording.Session.ID] = true
			if q.Day == "" || q.Day == recording.Session.StartedAt.UTC().Format("2006-01-02") {
				h.Sessions = append(h.Sessions, HistorySession{RecordingID: recording.Session.ID, Scope: "", Agent: "unknown", Projects: []string{"unknown"}, First: recording.Session.StartedAt, Last: recording.Session.StartedAt, Live: recording.Live, Ended: !recording.Session.EndedAt.IsZero(), RecoveredTail: recording.Session.RecoveredTail, UsageMetrics: usage(nil)})
			}
		}
		for scope, rows := range groups {
			first, last := rows[0].StartedAt, rows[0].StartedAt
			projectSet := map[string]bool{}
			for _, row := range rows {
				_, project := dimension(row)
				projectSet[project] = true
				if row.StartedAt.Before(first) {
					first = row.StartedAt
				}
				if row.StartedAt.After(last) {
					last = row.StartedAt
				}
			}
			agent, _ := dimension(rows[0])
			native := ""
			if e := Evidence(rows[0]); e != nil {
				native = e.AgentSessionID
			}
			h.Sessions = append(h.Sessions, HistorySession{RecordingID: recording.Session.ID, Scope: scope, Agent: agent, NativeSessionID: native, Projects: sortedKeys(projectSet), First: first, Last: last, Live: recording.Live, Ended: !recording.Session.EndedAt.IsZero(), RecoveredTail: recording.Session.RecoveredTail, UsageMetrics: usage(rows)})
		}
	}
	h.Totals, h.Previous = usage(current), usage(previous)
	h.RecordingCount, h.NativeSessions = len(recordingIDs), len(nativeIDs)
	h.Agents, h.Projects, h.Tools = breakdown(agents), breakdown(projects), breakdown(tools)
	h.AvailableAgents, h.AvailableProjects = sortedKeys(availableAgents), sortedKeys(availableProjects)
	for day := q.From.UTC().Truncate(24 * time.Hour); day.Before(q.To); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		h.Days = append(h.Days, Breakdown{key, usage(days[key])})
	}
	sort.Slice(h.Sessions, func(i, j int) bool {
		a, b := h.Sessions[i], h.Sessions[j]
		if !a.Last.Equal(b.Last) {
			return a.Last.After(b.Last)
		}
		if a.RecordingID != b.RecordingID {
			return a.RecordingID < b.RecordingID
		}
		return a.Scope < b.Scope
	})
	return h
}
