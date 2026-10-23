package engine_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
)

var at = time.Date(2026, 11, 2, 14, 0, 0, 0, time.UTC)

func samples(rows ...[][]any) []engine.Sample {
	var out []engine.Sample
	for i, r := range rows {
		s := engine.Sample{T: at.Add(time.Duration(i) * 15 * time.Second), Cols: []string{"blocker", "waiting_s", "state"}, Rows: r}
		if r == nil {
			s.Gap = true
		}
		out = append(out, s)
	}
	return out
}

func blocking() *engine.Rule {
	return &engine.Rule{ID: "X", Kind: engine.KindSeries, Source: "locks", GroupBy: []string{"blocker"}, Measure: "waiting_s",
		Threshold: engine.Cond{Op: ">=", Param: "min"}, Params: map[string]float64{"min": 30}, Confidence: engine.Confirmed}
}

func TestSeriesEpisodes(t *testing.T) {
	res := engine.EvaluateSeries(blocking(), samples(
		[][]any{{51, "12.0", "a"}},                       // 14:00:00 below
		[][]any{{51, "31.5", "a"}, {60, int64(40), "b"}}, // 14:00:15 both start
		[][]any{{51, "46.0", "a"}},                       // 14:00:30 51 goes on, 60 ends (1 sample)
		nil,                                              // 14:00:45 gap: ends 51 even if it continues
		[][]any{{51, "76.0", "a"}},                       // 14:01:00 new episode for 51
		[][]any{},                                        // 14:01:15 nothing
	))
	if res.Status != engine.StatusFinding || res.Confidence != engine.Confirmed {
		t.Fatalf("status = %s/%s (%s)", res.Status, res.Confidence, res.Reason)
	}
	got := fmt.Sprint(res.Columns, len(res.Rows))
	if got != "[start end duration_s samples peak blocker waiting_s state] 3" {
		t.Fatalf("result = %s", got)
	}
	want := []string{
		"[2026-11-02T14:00:15Z 2026-11-02T14:00:30Z 15 2 46 51 46.0 a]",
		"[2026-11-02T14:00:15Z 2026-11-02T14:00:15Z 0 1 40 60 40 b]",
		"[2026-11-02T14:01:00Z 2026-11-02T14:01:00Z 0 1 76 51 76.0 a]",
	}
	for i, w := range want {
		if fmt.Sprint(res.Rows[i]) != w && !(i < 2 && fmt.Sprint(res.Rows[1-i]) == w) { // same start: either order
			t.Errorf("row %d = %v, want %s", i, res.Rows[i], w)
		}
	}
}

func TestSeriesSustainWhereAndStatus(t *testing.T) {
	r := blocking()
	r.Sustain = 2
	r.Where = []engine.Cond{{Column: "state", Op: "!=", Value: "idle"}}
	res := engine.EvaluateSeries(r, samples(
		[][]any{{51, 40, "a"}},                // 1 sample only: dropped by sustain
		[][]any{},                             //
		[][]any{{52, 40, "idle"}},             // filtered by where
		[][]any{{52, 40, "idle"}},             //
		[][]any{{53, 35, "a"}, {53, 50, "a"}}, // two rows, same group: the larger counts
		[][]any{{53, 36, "a"}},
	))
	if res.Status != engine.StatusFinding || len(res.Rows) != 1 || fmt.Sprint(res.Rows[0][:5]) != "[2026-11-02T14:01:00Z 2026-11-02T14:01:15Z 15 2 50]" {
		t.Errorf("got %s %v", res.Status, res.Rows)
	}
	if res := engine.EvaluateSeries(r, samples([][]any{{51, 1, "a"}})); res.Status != engine.StatusOK {
		t.Errorf("below threshold: %s", res.Status)
	}
	if res := engine.EvaluateSeries(r, samples(nil, nil)); res.Status != engine.StatusNoData || !strings.Contains(res.Reason, "locks") {
		t.Errorf("only gaps: %s (%s), want no data", res.Status, res.Reason)
	}
	r.Measure = "missing"
	if res := engine.EvaluateSeries(r, samples([][]any{{51, 40, "a"}})); res.Status != engine.StatusNoData || !strings.Contains(res.Reason, "missing") {
		t.Errorf("unknown column: %s (%s)", res.Status, res.Reason)
	}
}

func TestSeriesMeasures(t *testing.T) {
	cols := []string{"wait_type", "wait_ms", "workers", "max_workers", "interval_s"}
	one := func(r *engine.Rule, row ...[]any) engine.Result {
		return engine.EvaluateSeries(r, []engine.Sample{{T: at, Cols: cols, Rows: row}})
	}
	perS := &engine.Rule{Kind: engine.KindSeries, Source: "waits", GroupBy: []string{"wait_type"}, Measure: "wait_ms", PerS: true,
		Threshold: engine.Cond{Op: ">=", Param: "p"}, Params: map[string]float64{"p": 2000}}
	if res := one(perS, []any{"LCK", 45000.0, 0, 0, 15.0}, []any{"IO", 15000.0, 0, 0, 15.0}); len(res.Rows) != 1 || res.Rows[0][4] != 3000.0 {
		t.Errorf("per_s: %v", res.Rows) // 45000 ms in 15 s = 3000 ms/s; 1000 ms/s stays below
	}
	pct := &engine.Rule{Kind: engine.KindSeries, Source: "workers", Measure: "workers", Of: "max_workers",
		Threshold: engine.Cond{Op: ">=", Param: "p"}, Params: map[string]float64{"p": 90}}
	if res := one(pct, []any{"", 0, int64(700), int64(768), 15.0}); res.Status != engine.StatusFinding || res.Rows[0][4] != 91.1 {
		t.Errorf("of: %v", res.Rows)
	}
	count := &engine.Rule{Kind: engine.KindSeries, Source: "activity", GroupBy: []string{"wait_type"}, Measure: "count",
		Threshold: engine.Cond{Op: ">=", Param: "p"}, Params: map[string]float64{"p": 2}}
	if res := one(count, []any{"Lock"}, []any{"Lock"}, []any{"IO"}); len(res.Rows) != 1 || res.Rows[0][4] != 2.0 || res.Rows[0][5] != "Lock" {
		t.Errorf("count: %v", res.Rows)
	}
}

func TestSeriesValidation(t *testing.T) {
	r := blocking()
	r.Title, r.Impact, r.NextAction, r.Verify = engine.Text{"en": "x", "pt-BR": "x"}, engine.Text{"en": "x", "pt-BR": "x"}, engine.Text{"en": "x", "pt-BR": "x"}, engine.Text{"en": "x", "pt-BR": "x"}
	r.Engine, r.Severity = "pg", engine.High
	if err := r.Validate("pg", "X"); err != nil {
		t.Fatalf("valid rule: %v", err)
	}
	for name, broken := range map[string]func(r *engine.Rule){
		"no source":       func(r *engine.Rule) { r.Source = "" },
		"bad op":          func(r *engine.Rule) { r.Threshold.Op = "==" },
		"undefined param": func(r *engine.Rule) { r.Threshold.Param = "nope" },
		"bad where":       func(r *engine.Rule) { r.Where = []engine.Cond{{Column: "s", Op: ">"}} },
		"count per_s":     func(r *engine.Rule) { r.Measure, r.PerS = "count", true },
		"unknown kind":    func(r *engine.Rule) { r.Kind = "stream" },
	} {
		c := *r
		broken(&c)
		if c.Validate("pg", "X") == nil {
			t.Errorf("%s: want a validation error", name)
		}
	}
}
