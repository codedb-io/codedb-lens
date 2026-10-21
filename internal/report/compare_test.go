package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
)

func run(at time.Time, cov float64, ver string, st map[string]engine.Status) *Report {
	r := &Report{Tool: "codeDB Lens", ToolVersion: ver, GeneratedAt: at, Lang: "en",
		Server: engine.ServerInfo{Engine: "mssql", Host: "SRV1"}, Score: engine.Score{Value: 80, Letter: "B", Coverage: cov}}
	for id, s := range st {
		x := engine.Result{Rule: &engine.Rule{ID: id, Severity: engine.High, Title: engine.Text{"en": id, "pt-BR": id}}, Status: s}
		if s == engine.StatusFinding {
			x.Columns, x.Rows = []string{"n"}, [][]any{{1}}
		}
		r.Results = append(r.Results, x)
	}
	return r
}

func ids(l []RuleDiff) string {
	var s []string
	for _, d := range l {
		s = append(s, d.Rule.ID)
	}
	return strings.Join(s, ",")
}

func TestCompare(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	a := run(t0, 1, "0.5.0", map[string]engine.Status{
		"NEW": "ok", "FIXED": "finding", "STILL": "finding", "LOST": "finding", "GAINED": "no_data", "SAME": "ok", "GONE": "ok",
	})
	b := run(t0.Add(24*time.Hour), 0.8, "0.5.1", map[string]engine.Status{
		"NEW": "finding", "FIXED": "ok", "STILL": "finding", "LOST": "no_data", "GAINED": "ok", "SAME": "ok", "ADDED": "ok",
	})
	// order of arguments does not matter: the older run is "before"
	c, err := Compare("0.5.1", "en", b, a, "b.json", "a.json")
	if err != nil {
		t.Fatal(err)
	}
	if c.Before.File != "a.json" {
		t.Errorf("before = %s", c.Before.File)
	}
	for name, got := range map[string][2]string{
		"new":        {ids(c.New), "NEW"},
		"resolved":   {ids(c.Resolved), "FIXED"},
		"persisting": {ids(c.Persisting), "STILL"},
		"changed":    {ids(c.Changed), "ADDED,GAINED,GONE,LOST"}, // finding -> no_data is never "resolved"
	} {
		if got[0] != got[1] {
			t.Errorf("%s = %q, want %q", name, got[0], got[1])
		}
	}
	keys := map[string]bool{}
	for _, w := range c.Warnings {
		keys[w.Key] = true
	}
	if !keys["cmp_warn_coverage"] || !keys["cmp_warn_version"] || !keys["cmp_warn_catalog"] {
		t.Errorf("warnings = %+v", c.Warnings)
	}

	var h bytes.Buffer
	if err := c.HTML(&h); err != nil || !strings.Contains(h.String(), "Achados novos") || !strings.Contains(h.String(), "80%") {
		t.Fatalf("html: %v", err)
	}
	dir := t.TempDir()
	path, err := c.Save(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(strings.TrimSuffix(path, ".html") + ".json")
	var back Comparison
	if err := json.Unmarshal(raw, &back); err != nil || ids(back.New) != "NEW" {
		t.Errorf("json round trip: %v", err)
	}
	// History must skip comparison files
	if _, err := a.Save(dir); err != nil {
		t.Fatal(err)
	}
	if h := History(dir, 10); len(h) != 1 {
		t.Errorf("history = %d entries, want 1", len(h))
	}
	if _, err := Load(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("load of a missing file")
	}

	b.Server.Host = "SRV2"
	if _, err := Compare("0.5.1", "en", a, b, "", ""); !errors.Is(err, ErrDifferentInstances) {
		t.Errorf("different instances: %v", err)
	}
}
