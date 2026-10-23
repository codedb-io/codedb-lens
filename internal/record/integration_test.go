//go:build integration

// SQL Server 2017 self-signed certificates have negative serial numbers, which Go rejects by default.
//go:debug x509negativeserial=1

// Runs every collector against the docker-compose databases (or LENS_MSSQL_SERVER / LENS_PG_DSN in CI):
// the SQL must run on each version of the matrix, deltas must work, and a provoked block must be recorded.
package record_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/record"
	"github.com/codedb-io/codedb-lens/rules"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func unreachable(t *testing.T, what string, err error) {
	if os.Getenv("LENS_REQUIRE_DB") != "" {
		t.Fatal(what, "not reachable:", err)
	}
	t.Skip(what, "not reachable:", err)
}

// collect runs one collector the way the recorder will: query, then Process.
func collect(t *testing.T, s adapter.Session, c *record.Collector, at time.Time) (record.Sample, bool, [][]any, []string) {
	t.Helper()
	cols, rows, err := s.Query(context.Background(), c.Query(s.Info().Platform))
	if err != nil {
		t.Fatalf("%s: %v", c.Name, err)
	}
	raw := slices.Clone(rows)
	smp, ok, err := c.Process(at, cols, rows)
	if err != nil {
		t.Fatalf("%s: %v", c.Name, err)
	}
	return smp, ok, raw, cols
}

// everyCollector runs each collector twice (deltas need a baseline) and checks the text query of the slow ones.
func everyCollector(t *testing.T, s adapter.Session, eng string, between func()) {
	cs, err := record.LoadCollectors(rules.FS, eng)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, c := range cs {
		t.Run(c.Name, func(t *testing.T) {
			if c.Skips(s.Info().Platform) {
				t.Skip("not collected on", s.Info().Platform)
			}
			_, ok, raw, cols := collect(t, s, c, now)
			if ok == (c.Mode == "delta") {
				t.Errorf("first run: ok = %v for a %s collector", ok, c.Mode)
			}
			between()
			smp, ok, _, _ := collect(t, s, c, now.Add(record.FastInterval))
			if !ok {
				t.Error("second run produced no sample")
			}
			// every series rule reading this collector finds its columns on this server version
			series, _ := engine.LoadSeries(rules.FS, eng)
			for _, r := range series {
				if r.Source != c.Name || r.Skips(s.Info().Platform) {
					continue
				}
				res := engine.EvaluateSeries(r, []engine.Sample{{T: now, Cols: smp.Cols, Rows: smp.Rows}})
				if res.Status == engine.StatusNoData {
					t.Errorf("%s: %s", r.ID, res.Reason)
				}
			}
			if c.TextSQL == "" {
				return
			}
			hi := slices.Index(cols, c.Hash)
			var hashes []string
			for _, r := range raw[:min(len(raw), 5)] {
				hashes = append(hashes, r[hi].(string))
			}
			if len(hashes) == 0 {
				t.Fatal("no queries to fetch texts for")
			}
			q, err := c.TextQuery(hashes)
			if err != nil {
				t.Fatal(err)
			}
			tc, tr, err := s.Query(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}
			if len(record.Texts(tc, tr)) == 0 {
				t.Errorf("no texts for %v", hashes)
			}
		})
	}
}

// blocked checks that the locks collector records a provoked block, with the blocked query's text.
func blocked(t *testing.T, s adapter.Session, eng string, block func() (release func())) {
	cs, _ := record.LoadCollectors(rules.FS, eng)
	i := slices.IndexFunc(cs, func(c *record.Collector) bool { return c.Name == "locks" })
	release := block()
	time.Sleep(1500 * time.Millisecond) // the collector starts at 1 s of waiting
	smp, _, _, _ := collect(t, s, cs[i], time.Now())
	release()
	if len(smp.Rows) == 0 || len(smp.Texts) == 0 {
		t.Errorf("block not recorded: %d rows, %d texts", len(smp.Rows), len(smp.Texts))
	}
	series, _ := engine.LoadSeries(rules.FS, eng)
	id := strings.ToUpper(map[string]string{"mssql": "MSSQL", "pg": "PG"}[eng]) + "-REC-BLOCKING-001"
	j := slices.IndexFunc(series, func(r *engine.Rule) bool { return r.ID == id })
	r := *series[j]
	r.Params = map[string]float64{"min_wait_s": 1}
	if res := engine.EvaluateSeries(&r, []engine.Sample{{T: time.Now(), Cols: smp.Cols, Rows: smp.Rows}}); res.Status != engine.StatusFinding {
		t.Errorf("%s: %s (%s), want a finding", id, res.Status, res.Reason)
	}
}

func TestMSSQLCollectors(t *testing.T) {
	ctx := context.Background()
	server, pw := env("LENS_MSSQL_SERVER", "localhost,14322"), env("MSSQL_SA_PASSWORD", "Lens_dev_Passw0rd!")
	s, err := adapter.Open(ctx, adapter.Conn{Engine: "mssql", Server: server, User: "sa", Password: pw, TrustServerCert: true})
	if err != nil {
		unreachable(t, "SQL Server", err)
	}
	defer s.Close()

	// Fixture writes only happen here, on the disposable test instance, through a separate connection.
	host, port, ok := strings.Cut(server, ",")
	if !ok {
		port = "1433"
	}
	u := url.URL{Scheme: "sqlserver", User: url.UserPassword("sa", pw), Host: host + ":" + port,
		RawQuery: "TrustServerCertificate=true"}
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec := func(q string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	exec(`IF DB_ID('lens_fixture') IS NULL CREATE DATABASE lens_fixture`)
	exec(`IF OBJECT_ID('lens_fixture.dbo.lock_t') IS NULL EXEC ('USE lens_fixture; CREATE TABLE dbo.lock_t (v int); INSERT dbo.lock_t VALUES (0)')`)

	everyCollector(t, s, "mssql", func() { exec(`SELECT COUNT(*) FROM sys.objects a CROSS JOIN sys.objects b`) })

	blocked(t, s, "mssql", func() func() {
		holder, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := holder.ExecContext(ctx, `BEGIN TRAN; UPDATE lens_fixture.dbo.lock_t SET v = v + 1;`); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { _, err := db.ExecContext(ctx, `SELECT v FROM lens_fixture.dbo.lock_t`); done <- err }()
		return func() {
			holder.ExecContext(ctx, `ROLLBACK`)
			holder.Close()
			if err := <-done; err != nil {
				t.Error(err)
			}
		}
	})
}

func TestPGCollectors(t *testing.T) {
	ctx := context.Background()
	dsn := env("LENS_PG_DSN", "postgres://postgres:lens_dev@localhost:15417/postgres?sslmode=disable")
	s, err := adapter.Open(ctx, adapter.Conn{Engine: "pg", DSN: dsn})
	if err != nil {
		unreachable(t, "PostgreSQL", err)
	}
	defer s.Close()

	fx, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer fx.Close(ctx)
	exec := func(q string) {
		t.Helper()
		if _, err := fx.Exec(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	exec(`CREATE EXTENSION IF NOT EXISTS pg_stat_statements`) // the containers preload it
	exec(`CREATE TABLE IF NOT EXISTS lens_lock (v int)`)

	everyCollector(t, s, "pg", func() { exec(`SELECT count(*) FROM generate_series(1, 100000)`) })

	blocked(t, s, "pg", func() func() {
		holder, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := holder.Exec(ctx, `BEGIN; LOCK TABLE lens_lock IN ACCESS EXCLUSIVE MODE;`); err != nil {
			t.Fatal(err)
		}
		waiter, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { _, err := waiter.Exec(ctx, `SELECT count(*) FROM lens_lock`); done <- err }()
		return func() {
			holder.Exec(ctx, `ROLLBACK`)
			if err := <-done; err != nil {
				t.Error(err)
			}
			holder.Close(ctx)
			waiter.Close(ctx)
		}
	})
}
