package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
)

func TestLocaltime(t *testing.T) {
	f := funcs["localtime"].(func(string) string)
	want := time.Date(2026, 10, 22, 15, 29, 6, 0, time.UTC).Local().Format("2006-01-02 15:04:05")
	if got := f("2026-10-22T15:29:06Z"); got != want {
		t.Errorf("UTC start: got %q, want %q", got, want)
	}
	if got := f("2026-10-22 15:29:06"); got != "2026-10-22 15:29:06" { // reports before v0.5
		t.Errorf("legacy start: got %q", got)
	}
}

// Counts in the HTML: total findings and critical/high are different numbers and must stay so.
func TestFindingCounts(t *testing.T) {
	txt := engine.Text{"en": "x", "pt-BR": "x"}
	f := func(sev engine.Severity) engine.Result {
		return engine.Result{Rule: &engine.Rule{ID: string(sev), Severity: sev, Title: txt, Impact: txt, NextAction: txt},
			Status: engine.StatusFinding, Confidence: engine.Confirmed}
	}
	r := New("test", "en", engine.ServerInfo{Engine: "mssql"}, engine.Score{Letter: "?", Limited: true, Applies: 2, Rules: 12},
		[]engine.Result{f(engine.High), f(engine.Low), f(engine.Low)})
	var b bytes.Buffer
	if err := r.HTML(&b); err != nil {
		t.Fatal(err)
	}
	for _, must := range []string{"Finding 3 of 3", "1 critical or high", "Only 2 of 12 rules apply"} {
		if !strings.Contains(b.String(), must) {
			t.Errorf("report missing %q", must)
		}
	}
}

func finding(id string, sev engine.Severity, conf engine.Confidence) engine.Result {
	txt := engine.Text{"en": id, "pt-BR": id}
	return engine.Result{Rule: &engine.Rule{ID: id, Severity: sev, Title: txt, Impact: txt, NextAction: txt}, Status: engine.StatusFinding, Confidence: conf}
}

// Summary top 3: severity, then Confirmed before Hint, then rule ID; the highest severity is the first one's.
func TestTopAndHighest(t *testing.T) {
	r := &Report{Results: []engine.Result{
		finding("Z", engine.Low, engine.Confirmed), finding("B", engine.High, engine.Hint), finding("C", engine.High, engine.Confirmed),
		finding("A", engine.High, engine.Hint), finding("D", engine.Medium, engine.Confirmed),
		{Rule: &engine.Rule{ID: "OK", Severity: engine.Critical}, Status: engine.StatusOK},
	}}
	var got []string
	for _, x := range r.Top(3) {
		got = append(got, x.Rule.ID)
	}
	if want := "C A B"; strings.Join(got, " ") != want {
		t.Errorf("top = %v, want %s", got, want)
	}
	if r.Highest() != engine.High {
		t.Errorf("highest = %q", r.Highest())
	}
	if (&Report{}).Highest() != "" || len((&Report{}).Top(3)) != 0 {
		t.Error("no findings: no top, no highest")
	}
}

// Evolution: no earlier run = first; earlier run without key = locked (CTA); with key = compared counts.
func TestSetEvolution(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	cur := run(t0.Add(48*time.Hour), 1, "0.6.0", map[string]engine.Status{"A": "ok", "B": "finding", "C": "finding"})
	html := func() string {
		var b bytes.Buffer
		if err := cur.HTML(&b); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}

	cur.SetEvolution(dir, true)
	if cur.Evolution.State != "first" || !strings.Contains(html(), "First diagnosis") {
		t.Errorf("empty folder: %+v", cur.Evolution)
	}

	prev := run(t0, 0.9, "0.5.1", map[string]engine.Status{"A": "finding", "B": "finding", "C": "ok"})
	if _, err := prev.Save(dir); err != nil {
		t.Fatal(err)
	}
	other := run(t0.Add(24*time.Hour), 1, "0.5.1", nil) // another host must be ignored
	other.Server.Host = "OTHER"
	if _, err := other.Save(dir); err != nil {
		t.Fatal(err)
	}

	cur.SetEvolution(dir, false)
	if cur.Evolution.State != "locked" || !strings.Contains(html(), "utm_medium=summary") || !strings.Contains(html(), "Compare with the previous diagnosis: free key") {
		t.Errorf("no key: %+v", cur.Evolution)
	}

	cur.SetEvolution(dir, true)
	if e := cur.Evolution; e.State != "compared" || e.PrevCoverage != 0.9 || e.New != 1 || e.Resolved != 1 {
		t.Errorf("with key: %+v", e)
	}
	if !strings.Contains(html(), "1 new · 1 resolved") {
		t.Error("evolution line missing from the HTML")
	}
}
