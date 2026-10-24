package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/internal/report"
	"github.com/codedb-io/codedb-lens/internal/saas"
)

const (
	previewRows = 5   // evidence rows per finding in check; get_finding has the rest
	maxRows     = 200 // evidence rows in get_finding
)

func addDiagTools(m *mcp.Server, s *server, ro *mcp.ToolAnnotations) {
	mcp.AddTool(m, &mcp.Tool{Name: "check", Annotations: ro,
		Description: "Run the codeDB Lens health check on a profile (read-only, 10-60 s): A-F grade with its coverage, findings with severity, confidence, evidence preview and next action, and rules without data. Saves the report in the profile's history, like the app does."},
		s.check)
	mcp.AddTool(m, &mcp.Tool{Name: "get_finding", Annotations: ro,
		Description: "Full detail of one rule in a saved run: status, impact, next action, how to verify the fix, and all evidence rows. Default run: the latest."},
		s.getFinding)
	mcp.AddTool(m, &mcp.Tool{Name: "history", Annotations: ro,
		Description: "Saved check runs of a profile, newest first: date, grade, coverage and number of findings. Each run's path can be passed to get_finding or compare."},
		s.history)
	mcp.AddTool(m, &mcp.Tool{Name: "compare", Annotations: ro,
		Description: "Compare two runs of the same instance: new, resolved and persisting findings and the grade change. Default: the two latest runs. Needs the user's free key (codedb-lens register)."},
		s.compare)
}

type profileIn struct {
	Profile string `json:"profile" jsonschema:"profile name from list_profiles"`
}

type checkIn struct {
	profileIn
	Redact bool `json:"redact,omitempty" jsonschema:"mask host, database, object, login and file names and drop query text"`
}

type serverOut struct {
	Engine   string `json:"engine"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Host     string `json:"host"`
}

type findingOut struct {
	ID         string   `json:"id"`
	Severity   string   `json:"severity"`
	Confidence string   `json:"confidence"`
	Title      string   `json:"title"`
	NextAction string   `json:"next_action"`
	DocURL     string   `json:"doc_url"`
	Columns    []string `json:"columns,omitempty"`
	Rows       [][]any  `json:"rows,omitempty"`
	TotalRows  int      `json:"total_rows"`
}

type noDataOut struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Reason string `json:"reason,omitempty"`
}

type checkOut struct {
	Server   serverOut    `json:"server"`
	Grade    string       `json:"grade"` // empty when too few rules apply to the platform
	Score    float64      `json:"score"`
	Coverage float64      `json:"coverage"` // 0-1: rules executed / applicable
	Findings []findingOut `json:"findings"`
	NoData   []noDataOut  `json:"no_data"`
	Passed   int          `json:"passed"`
	Report   string       `json:"report"` // HTML report on the user's computer
	Note     string       `json:"note"`
}

func (s *server) check(ctx context.Context, _ *mcp.CallToolRequest, in checkIn) (*mcp.CallToolResult, checkOut, error) {
	var out checkOut
	p, err := s.profile(in.Profile)
	if err != nil {
		return nil, out, err
	}
	c, err := s.conn(p)
	if err != nil {
		return nil, out, err
	}
	lang := s.lang(p)
	rep, path, err := s.Check(ctx, p, c, lang, in.Redact || s.Redact)
	if err != nil {
		return nil, out, err
	}
	i := rep.Server
	out = checkOut{Server: serverOut{i.Engine, i.Version, i.Platform, i.Host}, Score: rep.Score.Value,
		Coverage: rep.Score.Coverage, Findings: []findingOut{}, NoData: []noDataOut{}, Report: path,
		Note: fmt.Sprintf(i18n.T(lang, "cli_cta"), saas.Link(saas.Landing, "mcp", "check"))}
	if !rep.Score.Limited {
		out.Grade = rep.Score.Letter
	}
	for _, x := range rep.Results {
		switch x.Status {
		case engine.StatusFinding:
			f := finding(x, lang)
			f.Rows = f.Rows[:min(len(f.Rows), previewRows)]
			out.Findings = append(out.Findings, f)
		case engine.StatusNoData:
			out.NoData = append(out.NoData, noDataOut{x.Rule.ID, x.Rule.Title[lang], x.Reason})
		case engine.StatusOK:
			out.Passed++
		}
	}
	return nil, out, nil
}

func finding(x engine.Result, lang string) findingOut {
	r := x.Rule
	return findingOut{r.ID, string(r.Severity), string(x.Confidence), r.Title[lang], r.NextAction[lang], r.DocURL,
		x.Columns, x.Rows, len(x.Rows)}
}

type getFindingIn struct {
	profileIn
	RuleID string `json:"rule_id" jsonschema:"rule ID from check, e.g. MSSQL-BACKUP-001"`
	Run    string `json:"run,omitempty" jsonschema:"run path from history; default: the latest run"`
}

type getFindingOut struct {
	findingOut
	Status    engine.Status `json:"status"`
	Impact    string        `json:"impact"`
	Verify    string        `json:"verify"`
	Caveat    string        `json:"caveat,omitempty"`
	Reason    string        `json:"reason,omitempty"`
	Truncated bool          `json:"truncated,omitempty"`
	Run       string        `json:"run"`
	RunAt     time.Time     `json:"run_at"`
}

func (s *server) getFinding(_ context.Context, _ *mcp.CallToolRequest, in getFindingIn) (*mcp.CallToolResult, getFindingOut, error) {
	var out getFindingOut
	p, err := s.profile(in.Profile)
	if err != nil {
		return nil, out, err
	}
	run := in.Run
	if run == "" {
		h := report.History(reportsDir(p), 1)
		if len(h) == 0 {
			return nil, out, fmt.Errorf(i18n.T(s.Lang, "mcp_no_runs"), p.Name)
		}
		run = h[0].Path
	}
	file, err := s.runFile(p, run)
	if err != nil {
		return nil, out, err
	}
	rep, err := report.Load(file)
	if err != nil {
		return nil, out, err
	}
	if s.Redact && !rep.Redacted {
		salt, err := profile.RedactSalt()
		if err != nil {
			return nil, out, err
		}
		rep.Redact(salt)
	}
	for _, x := range rep.Results {
		if strings.EqualFold(x.Rule.ID, in.RuleID) {
			r, l := x.Rule, rep.Lang
			out = getFindingOut{findingOut: finding(x, l), Status: x.Status, Impact: r.Impact[l], Verify: r.Verify[l],
				Caveat: r.Caveat[l], Reason: x.Reason, Truncated: len(x.Rows) > maxRows, Run: run, RunAt: rep.GeneratedAt}
			out.Rows = out.Rows[:min(len(out.Rows), maxRows)]
			return nil, out, nil
		}
	}
	return nil, out, fmt.Errorf(i18n.T(s.Lang, "mcp_rule_not_in_run"), in.RuleID)
}

// runFile maps a run path (HTML or JSON, as history returns it) to its JSON report, only inside the
// profile's folder: the AI must not make the server read arbitrary files.
func (s *server) runFile(p profile.Profile, run string) (string, error) {
	file := strings.TrimSuffix(filepath.Clean(run), filepath.Ext(run)) + ".json"
	dir := filepath.Clean(reportsDir(p))
	if !strings.EqualFold(filepath.Dir(file), dir) || strings.HasPrefix(filepath.Base(file), "compare_") {
		return "", fmt.Errorf(i18n.T(s.Lang, "mcp_run_unknown"), run, p.Name)
	}
	return file, nil
}

type historyIn struct {
	profileIn
	Limit int `json:"limit,omitempty" jsonschema:"how many runs, newest first (default 10, at most 50)"`
}

type runOut struct {
	When     time.Time `json:"when"`
	Grade    string    `json:"grade"`
	Score    float64   `json:"score"`
	Coverage float64   `json:"coverage"`
	Findings int       `json:"findings"`
	Run      string    `json:"run"`
}

type historyOut struct {
	Runs []runOut `json:"runs"`
}

func (s *server) history(_ context.Context, _ *mcp.CallToolRequest, in historyIn) (*mcp.CallToolResult, historyOut, error) {
	out := historyOut{Runs: []runOut{}}
	p, err := s.profile(in.Profile)
	if err != nil {
		return nil, out, err
	}
	n := in.Limit
	if n <= 0 {
		n = 10
	}
	for _, h := range report.History(reportsDir(p), min(n, 50)) {
		out.Runs = append(out.Runs, runOut{h.When, h.Letter, h.Value, h.Coverage, h.Findings, h.Path})
	}
	return nil, out, nil
}

type compareIn struct {
	profileIn
	A string `json:"a,omitempty" jsonschema:"older run path from history; default: the second latest"`
	B string `json:"b,omitempty" jsonschema:"newer run path from history; default: the latest"`
}

type sideOut struct {
	When     time.Time `json:"when"`
	Grade    string    `json:"grade"`
	Score    float64   `json:"score"`
	Coverage float64   `json:"coverage"`
}

type diffOut struct {
	ID         string        `json:"id"`
	Severity   string        `json:"severity"`
	Title      string        `json:"title"`
	Before     engine.Status `json:"before,omitempty"`
	After      engine.Status `json:"after,omitempty"`
	RowsBefore int           `json:"rows_before"`
	RowsAfter  int           `json:"rows_after"`
}

type compareOut struct {
	Before     sideOut   `json:"before"`
	After      sideOut   `json:"after"`
	New        []diffOut `json:"new"`
	Resolved   []diffOut `json:"resolved"`
	Persisting []diffOut `json:"persisting"`
	Changed    []diffOut `json:"changed"` // started or stopped running; never counted as resolved
	Warnings   []string  `json:"warnings,omitempty"`
	Report     string    `json:"report"`
}

func (s *server) compare(_ context.Context, _ *mcp.CallToolRequest, in compareIn) (*mcp.CallToolResult, compareOut, error) {
	var out compareOut
	p, err := s.profile(in.Profile)
	if err != nil {
		return nil, out, err
	}
	lang := s.lang(p)
	a, b := in.A, in.B
	if a == "" && b == "" {
		var ok bool
		if a, b, ok = report.LatestTwo(reportsDir(p)); !ok {
			return nil, out, fmt.Errorf("%s", i18n.T(lang, "cmp_need_two"))
		}
	}
	if a, err = s.runFile(p, a); err != nil {
		return nil, out, err
	}
	if b, err = s.runFile(p, b); err != nil {
		return nil, out, err
	}
	c, path, err := s.Compare(lang, a, b)
	if err != nil {
		return nil, out, err
	}
	side := func(x report.Side) sideOut {
		return sideOut{x.GeneratedAt, x.Score.Letter, x.Score.Value, x.Score.Coverage}
	}
	diffs := func(l []report.RuleDiff) []diffOut {
		out := []diffOut{}
		for _, d := range l {
			out = append(out, diffOut{d.Rule.ID, string(d.Rule.Severity), d.Rule.Title[lang], d.Before, d.After, d.RowsBefore, d.RowsAfter})
		}
		return out
	}
	out = compareOut{side(c.Before), side(c.After), diffs(c.New), diffs(c.Resolved), diffs(c.Persisting), diffs(c.Changed), nil, path}
	for _, w := range c.Warnings {
		out.Warnings = append(out.Warnings, w.Text(lang))
	}
	return nil, out, nil
}
