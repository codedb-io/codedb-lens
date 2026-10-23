// demo writes the sample report in docs/<lang>/demo/ from synthetic data: no real server or customer data.
// Usage (from the repository root): go run ./tools/demo
package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/report"
	"github.com/codedb-io/codedb-lens/rules"
)

type fake struct {
	status engine.Status
	conf   engine.Confidence
	cols   []string
	rows   [][]any
	reason string
}

// Findings of every kind, two rules without data and the rest OK, so the sample shows each state.
var demo = map[string]fake{
	"MSSQL-BACKUP-001": {status: engine.StatusFinding, cols: []string{"database", "recovery_model", "last_full", "last_diff", "last_log", "problem"},
		rows: [][]any{
			{"sales", "FULL", "2026-10-12 23:10", nil, "2026-10-12 23:40", "full/diff too old"},
			{"inventory", "FULL", "2026-10-21 23:05", "2026-10-21 23:30", "2026-10-21 02:15", "log backup too old"},
		}},
	"MSSQL-CHECKDB-001": {status: engine.StatusFinding, cols: []string{"database", "last_good_checkdb", "days_ago", "create_date"},
		rows: [][]any{{"sales", "2026-08-30 01:00", 53, "2021-03-02"}, {"reporting", nil, nil, "2025-11-18"}}},
	"MSSQL-CONFIG-001": {status: engine.StatusFinding, conf: engine.Hint, cols: []string{"setting", "current_value", "suggested", "host"},
		rows: [][]any{
			{"max server memory (MB)", "2147483647", "26214", "32768 MB RAM"},
			{"cost threshold for parallelism", "5", "50", "16 CPUs"},
		}},
	"MSSQL-WAITS-001": {status: engine.StatusFinding, conf: engine.Hint,
		cols: []string{"meaning@en", "meaning@pt-BR", "wait_type", "share_pct", "wait_s_per_hour", "avg_wait_ms"},
		rows: [][]any{
			{"Disk reads (data pages not in memory)", "Leitura de disco (páginas fora da memória)", "PAGEIOLATCH_SH", 41.7, 812.4, 18.3},
			{"Parallel query coordination", "Coordenação de consultas paralelas", "CXPACKET", 22.9, 446.0, 3.1},
		}},
	"MSSQL-MISSINGIDX-001": {status: engine.StatusFinding, conf: engine.Hint,
		cols: []string{"database", "table", "equality_columns", "inequality_columns", "included_columns", "uses", "est_impact_pct", "improvement", "last_user_seek"},
		rows: [][]any{{"sales", "[sales].[dbo].[orders]", "[customer_id]", "[order_date]", "[total]", 48210, 93.4, 1840221, "2026-10-22 09:12"}}},
	"MSSQL-AGENT-001":     {status: engine.StatusNoData, reason: "The SELECT permission was denied on the object 'sysjobhistory', database 'msdb'."},
	"MSSQL-UNUSEDIDX-001": {status: engine.StatusNoData, reason: "skipped databases (no access): archive"},
}

// older is the run a week before: one finding since resolved, one not there yet, one rule that ran then and not now.
func older() map[string]fake {
	m := map[string]fake{}
	for id, f := range demo {
		m[id] = f
	}
	delete(m, "MSSQL-MISSINGIDX-001") // new in the latest run
	delete(m, "MSSQL-AGENT-001")      // OK then, no data now: "changed", never "resolved"
	m["MSSQL-TOPQUERY-001"] = fake{status: engine.StatusFinding,
		cols: []string{"cpu_pct", "executions", "avg_cpu_ms", "avg_duration_ms", "avg_reads", "database", "query"},
		rows: [][]any{{78.2, 1204331, 41.6, 55.0, 18220, "sales", "SELECT o.id, o.total FROM dbo.orders o WHERE o.status = @p0 ORDER BY o.order_date DESC"}}}
	return m
}

func main() {
	catalog, err := engine.LoadCatalog(rules.FS, "mssql")
	check(err)
	scoring, err := engine.LoadScoring(rules.FS)
	check(err)
	version, err := engine.CatalogVersion(rules.FS, "mssql")
	check(err)
	info := engine.ServerInfo{Engine: "mssql", Version: "16.0.4135.4", Major: 16, Platform: "onprem", Host: "sql-demo-01"}

	run := func(m map[string]fake, at time.Time, lang string) *report.Report {
		var results []engine.Result
		for _, r := range catalog {
			f, ok := m[r.ID]
			if !ok {
				f.status = engine.StatusOK
			}
			if f.conf == "" && f.status == engine.StatusFinding {
				f.conf = engine.Confirmed
			}
			results = append(results, engine.Result{Rule: r, Status: f.status, Confidence: f.conf, Columns: f.cols,
				Rows: f.rows, Reason: f.reason, Duration: "120ms", CollectedAt: at})
		}
		rep := report.New("0.6.0", lang, info, scoring.Compute(results), results)
		rep.GeneratedAt, rep.CatalogVersion, rep.Duration = at, version, "14s"
		return rep
	}

	at := time.Date(2026, 10, 22, 9, 30, 0, 0, time.UTC)
	for _, lang := range engine.Languages {
		before, after := run(older(), at.AddDate(0, 0, -7), lang), run(demo, at, lang)
		dir := filepath.Join("docs", lang, "demo")
		check(os.MkdirAll(dir, 0o755))
		cmp, err := report.Compare("0.6.0", lang, before, after, "2026-10-15_093000.json", "2026-10-22_093000.json")
		check(err)
		cmp.GeneratedAt = at
		// The demo shows the summary as a key holder sees it: compared with the run a week before.
		after.Evolution = &report.Evolution{State: "compared", PrevAt: before.GeneratedAt, PrevLetter: before.Score.Letter,
			PrevCoverage: before.Score.Coverage, New: len(cmp.New), Resolved: len(cmp.Resolved)}
		write(filepath.Join(dir, "report.html"), after.HTML)
		write(filepath.Join(dir, "compare.html"), cmp.HTML)
		if lang == "en" {
			write(filepath.Join(dir, "report.json"), after.JSON)
		}
	}
}

func write(path string, render func(io.Writer) error) {
	f, err := os.Create(path)
	check(err)
	check(render(f))
	check(f.Close())
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
