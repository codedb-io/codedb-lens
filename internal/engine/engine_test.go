package engine_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codedb-monitorsql/codedb-lens/internal/engine"
	"github.com/codedb-monitorsql/codedb-lens/internal/report"
	"github.com/codedb-monitorsql/codedb-lens/rules"
)

// The built-in catalog must load and pass validation (both languages, params, ID = folder).
func TestCatalogIsValid(t *testing.T) {
	for _, e := range []string{"mssql", "pg"} {
		rs, err := engine.LoadCatalog(rules.FS, e)
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) == 0 {
			t.Fatalf("no %s rules", e)
		}
		for _, r := range rs { // every rule documents itself in both languages
			for _, l := range engine.Languages {
				if _, err := os.Stat(filepath.Join("..", "..", "docs", l, "rules", strings.ToLower(r.ID)+".md")); err != nil {
					t.Errorf("%s: missing doc page in %s", r.ID, l)
				}
			}
		}
	}
	if _, err := engine.LoadScoring(rules.FS); err != nil {
		t.Fatal(err)
	}
}

type fake struct {
	info engine.ServerInfo
	resp map[string]func(ctx context.Context) ([]string, [][]any, error)
}

func (f fake) Info() engine.ServerInfo { return f.info }
func (f fake) Databases(context.Context) ([]string, []string, error) {
	return []string{"app", "broken"}, []string{"secret"}, nil
}
func (f fake) QueryDB(ctx context.Context, db, q string) ([]string, [][]any, error) {
	if db == "broken" {
		return nil, nil, errors.New("boom")
	}
	return f.Query(ctx, q)
}
func (f fake) Query(ctx context.Context, q string) ([]string, [][]any, error) {
	return f.resp[q](ctx)
}

func rule(id string, sev engine.Severity, sql string) *engine.Rule {
	txt := engine.Text{"en": "x", "pt-BR": "x"}
	return &engine.Rule{ID: id, Severity: sev, Confidence: engine.Confirmed, SQL: sql, Timeout: 50 * time.Millisecond,
		Title: txt, Impact: txt, NextAction: txt}
}

func TestRunStatusesAndScore(t *testing.T) {
	a := fake{info: engine.ServerInfo{Major: 16, Platform: "onprem"}, resp: map[string]func(context.Context) ([]string, [][]any, error){
		"ok":      func(context.Context) ([]string, [][]any, error) { return []string{"a"}, nil, nil },
		"finding": func(context.Context) ([]string, [][]any, error) { return []string{"db"}, [][]any{{"x"}}, nil },
		"hint": func(context.Context) ([]string, [][]any, error) {
			return []string{"db", "_confidence"}, [][]any{{"x", "hint"}}, nil
		},
		"denied": func(context.Context) ([]string, [][]any, error) { return nil, nil, errors.New("permission denied") },
		"slow": func(ctx context.Context) ([]string, [][]any, error) {
			<-ctx.Done()
			return nil, nil, ctx.Err()
		},
	}}
	old := rule("OLD", engine.High, "never")
	old.MaxMajor = 15
	rs := []*engine.Rule{
		rule("OK", engine.Critical, "ok"),         // w10 r1
		rule("FIND", engine.High, "finding"),      // w5 r0
		rule("HINT", engine.Medium, "hint"),       // w2 r0.5
		rule("DENIED", engine.Critical, "denied"), // no data
		rule("SLOW", engine.Low, "slow"),          // no data (timeout)
		old,                                       // not applicable
	}
	res := engine.Run(context.Background(), a, rs, nil)
	want := []engine.Status{engine.StatusOK, engine.StatusFinding, engine.StatusFinding, engine.StatusNoData, engine.StatusNoData, engine.StatusNotApplies}
	for i, r := range res {
		if r.Status != want[i] {
			t.Errorf("%s: status %s, want %s (%s)", r.Rule.ID, r.Status, want[i], r.Reason)
		}
	}
	if res[2].Confidence != engine.Hint || len(res[2].Columns) != 1 {
		t.Errorf("hint row: confidence %s, cols %v", res[2].Confidence, res[2].Columns)
	}
	if !strings.Contains(res[4].Reason, "timeout") {
		t.Errorf("slow rule reason = %q", res[4].Reason)
	}

	sc, _ := engine.LoadScoring(rules.FS)
	s := sc.Compute(res)
	// (10 + 0 + 1) / 17 = 64.7 -> C ; coverage 3/5 -> partial
	if s.Value != 64.7 || s.Letter != "C" || s.Executed != 3 || s.Applies != 5 || !s.Partial {
		t.Errorf("score = %+v", s)
	}

	var buf bytes.Buffer
	if err := report.New("test", "en", a.info, s, res).HTML(&buf); err != nil {
		t.Fatal(err)
	}
	for _, must := range []string{`lang="pt-BR"`, "utm_source=lens", "permission denied", "FIND"} {
		if !strings.Contains(buf.String(), must) {
			t.Errorf("report HTML missing %q", must)
		}
	}
}

func TestDatabaseScope(t *testing.T) {
	a := fake{resp: map[string]func(context.Context) ([]string, [][]any, error){
		"idx": func(context.Context) ([]string, [][]any, error) { return []string{"index"}, [][]any{{"ix1"}}, nil },
	}}
	r := rule("IDX", engine.Low, "idx")
	r.Scope = "database"
	res := engine.Run(context.Background(), a, []*engine.Rule{r}, nil)[0]
	if res.Status != engine.StatusFinding || res.Columns[0] != "database" || res.Rows[0][0] != "app" || len(res.Rows) != 1 {
		t.Fatalf("per-db result: %+v", res)
	}
	if !strings.Contains(res.Reason, "secret (no access)") || !strings.Contains(res.Reason, "broken (boom)") {
		t.Errorf("skipped databases not reported: %q", res.Reason)
	}
}

func TestRenderOnlyNumbers(t *testing.T) {
	r := rule("R", engine.Low, "SELECT {{n}} , {{ n }}")
	r.Params = map[string]float64{"n": 1.5}
	if got := r.Render(); got != "SELECT (1.5) , (1.5)" {
		t.Errorf("Render = %q", got)
	}
}
