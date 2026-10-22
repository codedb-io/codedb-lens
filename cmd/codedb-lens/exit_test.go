package main

import (
	"testing"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/report"
)

func TestExitCode(t *testing.T) {
	res := func(sev engine.Severity, st engine.Status) engine.Result {
		return engine.Result{Rule: &engine.Rule{Severity: sev}, Status: st}
	}
	r := &report.Report{
		Score:   engine.Score{Coverage: 0.5},
		Results: []engine.Result{res(engine.Critical, engine.StatusNoData), res(engine.Medium, engine.StatusFinding), res(engine.High, engine.StatusOK)},
	}
	cases := []struct {
		failOn engine.Severity
		minCov float64
		want   int
	}{
		{"", 0, 0},                        // no policy: findings never fail the run
		{engine.Medium, 0, exitFindings},  // finding at the threshold
		{engine.Low, 0, exitFindings},     // ... and above it
		{engine.High, 0, 0},               // a high rule with no data is not a finding
		{"", 80, exitCoverage},            // 50% < 80%
		{engine.Medium, 80, exitFindings}, // a finding wins over low coverage
		{engine.Critical, 50, 0},          // coverage exactly at the minimum passes
	}
	for _, c := range cases {
		if got := exitCode(r, c.failOn, c.minCov); got != c.want {
			t.Errorf("failOn=%q minCov=%v: got %d, want %d", c.failOn, c.minCov, got, c.want)
		}
	}
}
