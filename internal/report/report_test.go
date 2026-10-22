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
