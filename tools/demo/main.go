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

func main() {
	catalog, err := engine.LoadCatalog(rules.FS, "mssql")
	check(err)
	scoring, err := engine.LoadScoring(rules.FS)
	check(err)
	version, err := engine.CatalogVersion(rules.FS, "mssql")
	check(err)

	at := time.Date(2026, 10, 22, 9, 30, 0, 0, time.UTC)
	var results []engine.Result
	for _, r := range catalog {
		f, ok := demo[r.ID]
		if !ok {
			f.status = engine.StatusOK
		}
		if f.conf == "" && f.status == engine.StatusFinding {
			f.conf = engine.Confirmed
		}
		results = append(results, engine.Result{Rule: r, Status: f.status, Confidence: f.conf, Columns: f.cols,
			Rows: f.rows, Reason: f.reason, Duration: "120ms", CollectedAt: at})
	}
	info := engine.ServerInfo{Engine: "mssql", Version: "16.0.4135.4", Major: 16, Platform: "onprem", Host: "sql-demo-01"}

	for _, lang := range engine.Languages {
		rep := report.New("0.5.0", lang, info, scoring.Compute(results), results)
		rep.GeneratedAt, rep.CatalogVersion, rep.Duration = at, version, "14s"
		dir := filepath.Join("docs", lang, "demo")
		check(os.MkdirAll(dir, 0o755))
		write(filepath.Join(dir, "report.html"), rep.HTML)
		if lang == "en" {
			write(filepath.Join(dir, "report.json"), rep.JSON)
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
