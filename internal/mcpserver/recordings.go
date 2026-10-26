package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/internal/record"
	"github.com/codedb-io/codedb-lens/internal/report"
)

const (
	maxEpisodeRows = 20 // evidence rows per series finding
	maxTopQueries  = 10
)

func addRecordingTools(m *mcp.Server, s *server, ro *mcp.ToolAnnotations) {
	mcp.AddTool(m, &mcp.Tool{Name: "list_recordings", Annotations: ro,
		Description: "Recordings (activity sampled every 15 s for 1 or 24 h) kept with a profile's reports, newest first. Recordings made elsewhere can still be read with recording_report and their folder."},
		s.listRecordings)
	mcp.AddTool(m, &mcp.Tool{Name: "recording_report", Annotations: ro,
		Description: "Analyse a recording, optionally for a period (e.g. an incident): time coverage, gaps without data, findings as episodes (blocking, waits, long queries, workers or connections), peak of each chart and the top queries. Also writes the HTML report into the recording folder."},
		s.recordingReport)
	mcp.AddTool(m, &mcp.Tool{Name: "record_command", Annotations: ro,
		Description: "The command the user runs in a terminal (or on the server) to record a profile for up to 1 h, or 24 h with the free key. You cannot start a recording: give the user this command."},
		s.recordCommand)
}

type listRecordingsIn struct {
	Profile string `json:"profile,omitempty" jsonschema:"profile name; empty lists the recordings of every profile you may use"`
}

type recordingOut struct {
	Profile    string    `json:"profile"`
	Folder     string    `json:"folder"`
	Engine     string    `json:"engine"`
	Host       string    `json:"host,omitempty"`
	Started    time.Time `json:"started"`
	Stopped    time.Time `json:"stopped,omitzero"` // zero while it records
	StopReason string    `json:"stop_reason,omitempty"`
}

type listRecordingsOut struct {
	Recordings []recordingOut `json:"recordings"`
}

func (s *server) listRecordings(_ context.Context, _ *mcp.CallToolRequest, in listRecordingsIn) (*mcp.CallToolResult, listRecordingsOut, error) {
	out := listRecordingsOut{Recordings: []recordingOut{}}
	ps, err := s.all()
	if in.Profile != "" {
		var p profile.Profile
		p, err = s.profile(in.Profile)
		ps = []profile.Profile{p}
	}
	if err != nil {
		return nil, out, err
	}
	for _, p := range ps {
		for _, l := range record.List(recordingsDir(p)) {
			m := l.Meta
			r := recordingOut{p.Name, l.Dir, m.Server.Engine, m.Server.Host, m.StartedAt, m.StoppedAt, m.StopReason}
			if s.Redact {
				r.Host = ""
			}
			out.Recordings = append(out.Recordings, r)
		}
	}
	return nil, out, nil
}

type recordingReportIn struct {
	Folder string `json:"folder" jsonschema:"recording folder, from list_recordings or given by the user"`
	From   string `json:"from,omitempty" jsonschema:"start of the period, RFC 3339 (default: start of the recording)"`
	To     string `json:"to,omitempty" jsonschema:"end of the period, RFC 3339 (default: end of the recording)"`
}

type spanOut struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type chartOut struct {
	Title string  `json:"title"`
	Peak  float64 `json:"peak"`
}

type recordingReportOut struct {
	Server     serverOut    `json:"server"`
	Started    time.Time    `json:"started"`
	Stopped    time.Time    `json:"stopped,omitzero"`
	StopReason string       `json:"stop_reason,omitempty"`
	From       time.Time    `json:"from"`
	To         time.Time    `json:"to"`
	Coverage   float64      `json:"coverage"` // 0-1: 15 s cycles with data / expected
	Gaps       []spanOut    `json:"gaps"`
	Findings   []findingOut `json:"findings"`
	Charts     []chartOut   `json:"charts"`
	TopQueries report.Table `json:"top_queries"`
	Report     string       `json:"report"`
}

func (s *server) recordingReport(_ context.Context, _ *mcp.CallToolRequest, in recordingReportIn) (*mcp.CallToolResult, recordingReportOut, error) {
	var out recordingReportOut
	var from, to time.Time
	for _, x := range []struct {
		s string
		t *time.Time
	}{{in.From, &from}, {in.To, &to}} {
		if x.s == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, x.s)
		if err != nil {
			return nil, out, fmt.Errorf(i18n.T(s.Lang, "mcp_bad_time"), x.s)
		}
		*x.t = t
	}
	dir := filepath.Clean(in.Folder)
	tl, err := report.BuildTimeline(dir, from, to, s.Version, s.Lang)
	if err != nil {
		return nil, out, err
	}
	if s.Redact {
		salt, err := profile.RedactSalt()
		if err != nil {
			return nil, out, err
		}
		tl.Redact(salt)
	}
	path, err := tl.Save(dir, !from.IsZero() || !to.IsZero())
	if err != nil {
		return nil, out, err
	}
	m, i := tl.Recording, tl.Recording.Server
	out = recordingReportOut{Server: serverOut{i.Engine, i.Version, i.Platform, i.Host}, Started: m.StartedAt,
		Stopped: m.StoppedAt, StopReason: m.StopReason, From: tl.From, To: tl.To, Coverage: tl.Coverage,
		Gaps: []spanOut{}, Findings: []findingOut{}, Charts: []chartOut{}, TopQueries: tl.Top, Report: path}
	for _, g := range tl.Gaps {
		out.Gaps = append(out.Gaps, spanOut{g.From, g.To})
	}
	for _, x := range tl.Results {
		if x.Status == engine.StatusFinding {
			f := finding(x, s.Lang)
			f.Rows = f.Rows[:min(len(f.Rows), maxEpisodeRows)]
			out.Findings = append(out.Findings, f)
		}
	}
	for _, c := range tl.Charts {
		out.Charts = append(out.Charts, chartOut{i18n.T(s.Lang, c.Key), c.Max})
	}
	out.TopQueries.Rows = out.TopQueries.Rows[:min(len(out.TopQueries.Rows), maxTopQueries)]
	return nil, out, nil
}

type recordCommandIn struct {
	profileIn
	Duration string `json:"duration,omitempty" jsonschema:"how long to record, e.g. 30m, 1h or 24h (default 1h; more than 1h needs the free key)"`
}

type recordCommandOut struct {
	Command       string `json:"command"`        // on this computer, with the saved profile
	ServerCommand string `json:"server_command"` // on a server or in Docker, without profiles
	ReportCommand string `json:"report_command"`
	Note          string `json:"note"`
}

func (s *server) recordCommand(_ context.Context, _ *mcp.CallToolRequest, in recordCommandIn) (*mcp.CallToolResult, recordCommandOut, error) {
	var out recordCommandOut
	p, err := s.profile(in.Profile)
	if err != nil {
		return nil, out, err
	}
	d := time.Hour
	if in.Duration != "" {
		if d, err = time.ParseDuration(in.Duration); err != nil || d <= 0 || d > 24*time.Hour {
			return nil, out, fmt.Errorf(i18n.T(s.Lang, "mcp_bad_duration"), in.Duration)
		}
	}
	dur := d.String() // 1h0m0s -> 1h, 1h30m0s -> 1h30m; seconds stay
	if strings.HasSuffix(dur, "m0s") {
		dur = strings.TrimSuffix(dur, "0s")
	}
	if strings.HasSuffix(dur, "h0m") {
		dur = strings.TrimSuffix(dur, "0m")
	}
	c := p.Conn
	flags := "--engine " + c.Engine + " --server " + Quote(c.Server)
	if c.Port != 0 {
		flags += " --port " + strconv.Itoa(c.Port)
	}
	if c.Database != "" {
		flags += " --database " + Quote(c.Database)
	}
	if c.Auth == "windows" {
		flags += " --auth windows"
	} else if c.User != "" {
		flags += " --user " + Quote(c.User)
	}
	if c.TrustServerCert {
		flags += " --trust-server-cert"
	}
	if c.SSLMode != "" {
		flags += " --sslmode " + c.SSLMode
	}
	folder := filepath.Join(recordingsDir(p), record.DefaultDir(c.Server, time.Now()))
	out.ServerCommand = "codedb-lens record " + flags + " --duration " + dur
	out.Command = "codedb-lens record --profile " + Quote(p.Name) + " --duration " + dur + " --out " + Quote(folder)
	if s.Fixed != nil {
		out.Command = out.ServerCommand
	}
	out.ReportCommand = "codedb-lens report " + Quote(folder)
	out.Note = i18n.T(s.lang(p), "mcp_record_note")
	if d > time.Hour {
		out.Note += " " + i18n.T(s.lang(p), "rec_need_key")
	}
	return nil, out, nil
}

// Quote wraps values with spaces or shell characters; plain ones stay readable.
func Quote(v string) string {
	if v != "" && !strings.ContainsAny(v, " \t\"'\\$&|;<>()`*?!#") {
		return v
	}
	return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
}

func addPrompts(m *mcp.Server) {
	m.AddPrompt(&mcp.Prompt{Name: "diagnose", Title: "Diagnose a database",
		Description: "Run the health check on a profile and explain the biggest risks and what to do.",
		Arguments:   []*mcp.PromptArgument{{Name: "profile", Description: "profile name from list_profiles", Required: true}}},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return userPrompt(fmt.Sprintf("Run the codeDB Lens check on the profile %q. Explain the 3 biggest risks in plain words, "+
				"with the evidence, and what to do first for each (use get_finding for details). Mention rules without data and why.",
				req.Params.Arguments["profile"])), nil
		})
	m.AddPrompt(&mcp.Prompt{Name: "incident", Title: "Analyse an incident",
		Description: "Read a recording over a period and explain what happened.",
		Arguments: []*mcp.PromptArgument{
			{Name: "folder", Description: "recording folder (list_recordings)", Required: true},
			{Name: "from", Description: "start of the period, e.g. 2026-10-23T14:00:00-03:00"},
			{Name: "to", Description: "end of the period"},
		}},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			a := req.Params.Arguments
			return userPrompt(fmt.Sprintf("Use recording_report on the recording %q (from %q to %q; empty means the whole recording). "+
				"Tell me what happened, in time order: blocking, waits, long queries and the top queries, and whether gaps "+
				"without data limit the conclusion. Then suggest what to check or fix.", a["folder"], a["from"], a["to"])), nil
		})
	m.AddPrompt(&mcp.Prompt{Name: "explore_data", Title: "Explore a database",
		Description: "Describe the tables of a database and suggest questions the data can answer.",
		Arguments: []*mcp.PromptArgument{
			{Name: "profile", Description: "profile name from list_profiles", Required: true},
			{Name: "database", Description: "database name (list_databases); empty means the profile's"},
		}},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			a := req.Params.Arguments
			return userPrompt(fmt.Sprintf("On the profile %q, database %q (empty means the profile's), use list_tables and "+
				"describe_table on the main tables. Explain in plain words what the data is about and how the tables relate, "+
				"then suggest 5 questions it can answer, each with the SQL you would run.", a["profile"], a["database"])), nil
		})
}

// userPrompt is the message a prompt puts in the chat. The text is English, so it asks the AI not to switch the
// conversation to English: a slash command is often the only thing the user "said".
func userPrompt(text string) *mcp.GetPromptResult {
	text += " Answer in the language the user usually writes to you in, which may not be English."
	return &mcp.GetPromptResult{Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}}
}
