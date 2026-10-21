package report

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/i18n"
)

//go:embed compare.html.tmpl
var compareText string

// Side is one of the two runs being compared.
type Side struct {
	File        string            `json:"file"`
	GeneratedAt time.Time         `json:"generated_at"`
	ToolVersion string            `json:"tool_version"`
	Server      engine.ServerInfo `json:"server"`
	Score       engine.Score      `json:"score"`
}

// RuleDiff is one rule whose status differs between the runs (or that is still a finding).
type RuleDiff struct {
	Rule       *engine.Rule  `json:"rule"`
	Before     engine.Status `json:"before,omitempty"` // "" = the rule was not in that run
	After      engine.Status `json:"after,omitempty"`
	RowsBefore int           `json:"rows_before"`
	RowsAfter  int           `json:"rows_after"`
}

// Warning is an i18n key plus its arguments, so the HTML can show it in both languages.
type Warning struct {
	Key  string   `json:"key"`
	Args []string `json:"args,omitempty"`
}

type Comparison struct {
	Tool        string     `json:"tool"`
	ToolVersion string     `json:"tool_version"`
	GeneratedAt time.Time  `json:"generated_at"`
	Lang        string     `json:"lang"`
	Before      Side       `json:"before"`
	After       Side       `json:"after"`
	New         []RuleDiff `json:"new"`        // became a finding
	Resolved    []RuleDiff `json:"resolved"`   // finding -> ok
	Persisting  []RuleDiff `json:"persisting"` // finding in both
	Changed     []RuleDiff `json:"changed"`    // started/stopped running; never counted as resolved
	Warnings    []Warning  `json:"warnings,omitempty"`
}

// Load reads a JSON report written by Save.
func Load(path string) (*Report, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(raw, &r); err != nil || r.Tool != "codeDB Lens" {
		return nil, fmt.Errorf("%s is not a codeDB Lens JSON report", filepath.Base(path))
	}
	return &r, nil
}

// SameInstance reports whether two runs are from the same server (engine + host).
func SameInstance(a, b engine.ServerInfo) bool {
	return a.Engine == b.Engine && strings.EqualFold(a.Host, b.Host)
}

var ErrDifferentInstances = errors.New("the two reports are from different instances")

// Compare diffs two runs of the same instance; a is the older one.
func Compare(version, lang string, a, b *Report, fileA, fileB string) (*Comparison, error) {
	if !SameInstance(a.Server, b.Server) {
		return nil, fmt.Errorf("%w (%s %s, %s %s)", ErrDifferentInstances, a.Server.Engine, a.Server.Host, b.Server.Engine, b.Server.Host)
	}
	if b.GeneratedAt.Before(a.GeneratedAt) {
		a, b, fileA, fileB = b, a, fileB, fileA
	}
	c := &Comparison{Tool: "codeDB Lens", ToolVersion: version, GeneratedAt: time.Now(), Lang: lang,
		Before: side(a, fileA), After: side(b, fileB)}

	before, after := byID(a), byID(b)
	ids := map[string]bool{}
	for id := range before {
		ids[id] = true
	}
	catalogDiffers := len(before) != len(after)
	for id := range after {
		if !ids[id] {
			catalogDiffers = true
		}
		ids[id] = true
	}
	for id := range ids {
		x, y := before[id], after[id]
		d := RuleDiff{}
		if x != nil {
			d.Rule, d.Before, d.RowsBefore = x.Rule, x.Status, len(x.Rows)
		}
		if y != nil {
			d.Rule, d.After, d.RowsAfter = y.Rule, y.Status, len(y.Rows)
		}
		fb, fa := d.Before == engine.StatusFinding, d.After == engine.StatusFinding
		switch {
		case fa && fb:
			c.Persisting = append(c.Persisting, d)
		case fa:
			c.New = append(c.New, d)
		case fb && d.After == engine.StatusOK:
			c.Resolved = append(c.Resolved, d)
		case d.Before != d.After:
			c.Changed = append(c.Changed, d)
		}
	}
	for _, l := range [][]RuleDiff{c.New, c.Resolved, c.Persisting, c.Changed} {
		sortDiffs(l)
	}

	if math.Abs(a.Score.Coverage-b.Score.Coverage) > 0.05 {
		c.Warnings = append(c.Warnings, Warning{"cmp_warn_coverage", []string{pct(a.Score.Coverage), pct(b.Score.Coverage)}})
	}
	if a.ToolVersion != b.ToolVersion {
		c.Warnings = append(c.Warnings, Warning{"cmp_warn_version", []string{a.ToolVersion, b.ToolVersion}})
	}
	if catalogDiffers {
		c.Warnings = append(c.Warnings, Warning{Key: "cmp_warn_catalog"})
	}
	return c, nil
}

func side(r *Report, file string) Side {
	return Side{File: file, GeneratedAt: r.GeneratedAt, ToolVersion: r.ToolVersion, Server: r.Server, Score: r.Score}
}

func byID(r *Report) map[string]*engine.Result {
	m := make(map[string]*engine.Result, len(r.Results))
	for i := range r.Results {
		if x := &r.Results[i]; x.Rule != nil {
			m[x.Rule.ID] = x
		}
	}
	return m
}

func sortDiffs(l []RuleDiff) {
	sort.SliceStable(l, func(i, j int) bool {
		a, b := severityOrder[l[i].Rule.Severity], severityOrder[l[j].Rule.Severity]
		if a != b {
			return a < b
		}
		return l[i].Rule.ID < l[j].Rule.ID
	})
}

func pct(f float64) string { return fmt.Sprintf("%.0f%%", f*100) }

// Delta is the grade change in points, for the template and the CLI.
func (c *Comparison) Delta() float64 { return c.After.Score.Value - c.Before.Score.Value }

// Text renders a warning in one language (CLI).
func (w Warning) Text(lang string) string {
	args := make([]any, len(w.Args))
	for i, a := range w.Args {
		args[i] = a
	}
	return fmt.Sprintf(i18n.T(lang, w.Key), args...)
}

var compareTmpl = template.Must(template.New("compare").Funcs(funcs).Funcs(template.FuncMap{
	"warn": func(w Warning) template.HTML { return both(w.Text) },
	"st": func(s engine.Status) template.HTML {
		if s == "" {
			return both(func(l string) string { return i18n.T(l, "cmp_st_absent") })
		}
		return both(func(l string) string { return i18n.T(l, "cmp_st_"+string(s)) })
	},
	"signed": func(f float64) string { return fmt.Sprintf("%+.0f", f) },
	"base":   filepath.Base,
}).Parse(compareText))

func (c *Comparison) HTML(w io.Writer) error { return compareTmpl.Execute(w, c) }

func (c *Comparison) JSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

// Save writes <dir>/compare_<before>_vs_<after>.html and .json and returns the HTML path.
func (c *Comparison) Save(dir string) (string, error) {
	name := func(s Side) string { return s.GeneratedAt.Format("2006-01-02_150405") }
	base := filepath.Join(dir, "compare_"+name(c.Before)+"_vs_"+name(c.After))
	return base + ".html", writeAll(dir, base, map[string]func(io.Writer) error{".html": c.HTML, ".json": c.JSON})
}
