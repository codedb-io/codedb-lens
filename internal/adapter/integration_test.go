//go:build integration

// SQL Server 2017 self-signed certificates have negative serial numbers, which Go rejects by default.
//go:debug x509negativeserial=1

// Runs the real catalog against the docker-compose databases (or LENS_MSSQL_SERVER / LENS_PG_DSN in CI).
// Each rule is provoked (finding expected) and checked clean (OK expected).
package adapter

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/rules"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// unreachable skips locally but fails in CI (LENS_REQUIRE_DB=1), so a dead container can't pass silently.
func unreachable(t *testing.T, what string, err error) {
	if os.Getenv("LENS_REQUIRE_DB") != "" {
		t.Fatal(what, "not reachable:", err)
	}
	t.Skip(what, "not reachable:", err)
}

func catalog(t *testing.T, e string) map[string]*engine.Rule {
	rs, err := engine.LoadCatalog(rules.FS, e)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]*engine.Rule{}
	for _, r := range rs {
		m[r.ID] = r
	}
	return m
}

func expect(t *testing.T, s Session, r *engine.Rule, want engine.Status, conf engine.Confidence) {
	t.Helper()
	res := engine.Run(context.Background(), s, []*engine.Rule{r}, nil)[0]
	if res.Status != want || (conf != "" && res.Confidence != conf) {
		t.Errorf("%s: got %s/%s (%s), want %s/%s", r.ID, res.Status, res.Confidence, res.Reason, want, conf)
	}
}

func TestMSSQL(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, Conn{Engine: "mssql", Server: env("LENS_MSSQL_SERVER", "localhost,14322"), User: "sa",
		Password: env("MSSQL_SA_PASSWORD", "Lens_dev_Passw0rd!"), TrustServerCert: true})
	if err != nil {
		unreachable(t, "SQL Server", err)
	}
	defer s.Close()
	rs := catalog(t, "mssql")
	m := s.(*MSSQL)

	// Fixture setup is the only place writes happen, and only on the disposable test instance.
	exec := func(t *testing.T, q string) {
		t.Helper()
		if _, err := m.conn.ExecContext(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	exec(t, `IF DB_ID('lens_fixture') IS NULL CREATE DATABASE lens_fixture`)
	exec(t, `ALTER DATABASE lens_fixture SET RECOVERY FULL`)

	t.Run("MSSQL-BACKUP-001", func(t *testing.T) {
		exec(t, `EXEC msdb.dbo.sp_delete_database_backuphistory @database_name = 'lens_fixture'`)
		expect(t, s, rs["MSSQL-BACKUP-001"], engine.StatusFinding, engine.Confirmed)
		exec(t, `DECLARE @n sysname, @sql nvarchar(400);
			DECLARE c CURSOR LOCAL FOR SELECT name FROM sys.databases WHERE database_id <> 2 AND state_desc = 'ONLINE';
			OPEN c; FETCH c INTO @n;
			WHILE @@FETCH_STATUS = 0 BEGIN
				SET @sql = N'BACKUP DATABASE ' + QUOTENAME(@n) + N' TO DISK = ''NUL''';
				EXEC (@sql);
				IF DATABASEPROPERTYEX(@n, 'Recovery') <> 'SIMPLE' AND @n <> 'model'
					BEGIN SET @sql = N'BACKUP LOG ' + QUOTENAME(@n) + N' TO DISK = ''NUL'''; EXEC (@sql); END
				FETCH c INTO @n;
			END`)
		expect(t, s, rs["MSSQL-BACKUP-001"], engine.StatusOK, "")
	})

	t.Run("MSSQL-CHECKDB-001", func(t *testing.T) {
		r := *rs["MSSQL-CHECKDB-001"]
		r.Params = map[string]float64{"max_days": -1} // every database, even a fresh one, counts as overdue
		expect(t, s, &r, engine.StatusFinding, engine.Confirmed)
		exec(t, `DBCC CHECKDB ('master') WITH NO_INFOMSGS; DBCC CHECKDB ('model') WITH NO_INFOMSGS;
			DBCC CHECKDB ('msdb') WITH NO_INFOMSGS; DBCC CHECKDB ('lens_fixture') WITH NO_INFOMSGS;`)
		expect(t, s, rs["MSSQL-CHECKDB-001"], engine.StatusOK, "")
	})

	t.Run("MSSQL-AGENT-001", func(t *testing.T) {
		exec(t, `IF EXISTS (SELECT 1 FROM msdb.dbo.sysjobs WHERE name = 'lens_failing_job') EXEC msdb.dbo.sp_delete_job @job_name = 'lens_failing_job'`)
		expect(t, s, rs["MSSQL-AGENT-001"], engine.StatusOK, "")
		exec(t, `EXEC msdb.dbo.sp_add_job @job_name = 'lens_failing_job';
			EXEC msdb.dbo.sp_add_jobstep @job_name = 'lens_failing_job', @step_name = 'fail', @command = 'RAISERROR(''lens fixture'', 16, 1)';
			EXEC msdb.dbo.sp_add_jobserver @job_name = 'lens_failing_job';
			EXEC msdb.dbo.sp_start_job @job_name = 'lens_failing_job';`)
		for i := 0; i < 30; i++ { // the Agent runs the job asynchronously
			if res := engine.Run(context.Background(), s, []*engine.Rule{rs["MSSQL-AGENT-001"]}, nil)[0]; res.Status == engine.StatusFinding {
				break
			}
			time.Sleep(time.Second)
		}
		expect(t, s, rs["MSSQL-AGENT-001"], engine.StatusFinding, engine.Confirmed)
		exec(t, `EXEC msdb.dbo.sp_delete_job @job_name = 'lens_failing_job'`)
		expect(t, s, rs["MSSQL-AGENT-001"], engine.StatusOK, "")
	})

	t.Run("MSSQL-SPACE-001", func(t *testing.T) {
		r := *rs["MSSQL-SPACE-001"]
		r.Params = map[string]float64{"volume_hint_pct": 0, "volume_confirmed_pct": 0, "log_hint_pct": 75, "log_confirmed_pct": 90}
		expect(t, s, &r, engine.StatusOK, "") // log branch only: CI runner disks really are low on space
		// An open transaction in another session + BACKUP LOG -> log_reuse_wait = ACTIVE_TRANSACTION, deterministically.
		db, err := sql.Open("sqlserver", mssqlURL(Conn{Server: env("LENS_MSSQL_SERVER", "localhost,14322"), User: "sa",
			Password: env("MSSQL_SA_PASSWORD", "Lens_dev_Passw0rd!"), TrustServerCert: true}))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		holder, _ := db.Conn(ctx)
		defer holder.Close()
		exec(t, `BACKUP DATABASE lens_fixture TO DISK = 'NUL';
			IF OBJECT_ID('lens_fixture.dbo.big') IS NULL EXEC ('USE lens_fixture; CREATE TABLE dbo.big (v char(1000))');`)
		if _, err := holder.ExecContext(ctx, `BEGIN TRAN; INSERT lens_fixture.dbo.big VALUES ('x');
			INSERT lens_fixture.dbo.big SELECT TOP (20000) 'y' FROM sys.all_objects a CROSS JOIN sys.all_objects b;`); err != nil {
			t.Fatal(err)
		}
		defer holder.ExecContext(ctx, `ROLLBACK`)
		exec(t, `BACKUP LOG lens_fixture TO DISK = 'NUL'`)
		r.Params = map[string]float64{"volume_hint_pct": 0, "volume_confirmed_pct": 0, "log_hint_pct": 0, "log_confirmed_pct": 1000}
		expect(t, s, &r, engine.StatusFinding, engine.Hint)
		r.Params = map[string]float64{"volume_hint_pct": 101, "volume_confirmed_pct": 101, "log_hint_pct": 1000, "log_confirmed_pct": 1000}
		expect(t, s, &r, engine.StatusFinding, engine.Confirmed)
		exec(t, `BACKUP LOG lens_fixture TO DISK = 'NUL'`)
	})

	t.Run("MSSQL-CONFIG-001", func(t *testing.T) {
		exec(t, `EXEC sp_configure 'show advanced options', 1; RECONFIGURE;
			EXEC sp_configure 'cost threshold for parallelism', 5; EXEC sp_configure 'optimize for ad hoc workloads', 0; RECONFIGURE;`)
		expect(t, s, rs["MSSQL-CONFIG-001"], engine.StatusFinding, engine.Hint)
		exec(t, `EXEC sp_configure 'max server memory (MB)', 4096; EXEC sp_configure 'max degree of parallelism', 8;
			EXEC sp_configure 'cost threshold for parallelism', 50; EXEC sp_configure 'optimize for ad hoc workloads', 1; RECONFIGURE;`)
		expect(t, s, rs["MSSQL-CONFIG-001"], engine.StatusOK, "")
	})

	t.Run("MSSQL-TEMPDB-001", func(t *testing.T) {
		r := rs["MSSQL-TEMPDB-001"]
		hasPercent := func() bool {
			res := engine.Run(context.Background(), s, []*engine.Rule{r}, nil)[0]
			if res.Status == engine.StatusNoData {
				t.Fatal(res.Reason)
			}
			for _, row := range res.Rows {
				if row[0] == "growth in percent" {
					return true
				}
			}
			return false
		}
		exec(t, `ALTER DATABASE tempdb MODIFY FILE (NAME = templog, FILEGROWTH = 10%)`)
		if !hasPercent() {
			t.Error("percent growth not detected")
		}
		exec(t, `ALTER DATABASE tempdb MODIFY FILE (NAME = templog, FILEGROWTH = 64MB)`)
		if hasPercent() {
			t.Error("percent growth still reported after fixing it")
		}
	})

	// rowsOf runs one rule and returns its evidence rows; fails the test on No data.
	rowsOf := func(t *testing.T, r *engine.Rule) [][]any {
		t.Helper()
		res := engine.Run(context.Background(), s, []*engine.Rule{r}, nil)[0]
		if res.Status == engine.StatusNoData {
			t.Fatal(r.ID, res.Reason)
		}
		return res.Rows
	}
	mentions := func(rows [][]any, s string) bool {
		for _, row := range rows {
			for _, v := range row {
				if str, ok := v.(string); ok && strings.Contains(str, s) {
					return true
				}
			}
		}
		return false
	}

	t.Run("MSSQL-WAITS-001", func(t *testing.T) {
		r := *rs["MSSQL-WAITS-001"]
		r.Params = map[string]float64{"min_share_pct": 0, "min_wait_s_per_hour": 0}
		expect(t, s, &r, engine.StatusFinding, "")
		r.Params = map[string]float64{"min_share_pct": 101, "min_wait_s_per_hour": 0}
		expect(t, s, &r, engine.StatusOK, "")
	})

	t.Run("MSSQL-BLOCKING-001", func(t *testing.T) {
		r := *rs["MSSQL-BLOCKING-001"]
		r.Params = map[string]float64{"min_wait_ms": 0}
		expect(t, s, &r, engine.StatusOK, "")
		exec(t, `IF OBJECT_ID('lens_fixture.dbo.lock_t') IS NULL EXEC ('USE lens_fixture; CREATE TABLE dbo.lock_t (v int); INSERT dbo.lock_t VALUES (0)')`)

		db, err := sql.Open("sqlserver", mssqlURL(Conn{Server: env("LENS_MSSQL_SERVER", "localhost,14322"), User: "sa",
			Password: env("MSSQL_SA_PASSWORD", "Lens_dev_Passw0rd!"), TrustServerCert: true}))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		holder, _ := db.Conn(ctx)
		defer holder.Close()
		if _, err := holder.ExecContext(ctx, `BEGIN TRAN; UPDATE lens_fixture.dbo.lock_t SET v = v + 1;`); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { // blocked reader, released by the ROLLBACK below
			_, err := db.ExecContext(ctx, `SELECT v FROM lens_fixture.dbo.lock_t`)
			done <- err
		}()
		time.Sleep(1500 * time.Millisecond)
		rows := rowsOf(t, &r)
		if len(rows) == 0 || !mentions(rows, "sleeping") {
			t.Errorf("blocking not detected (or blocker not shown as sleeping): %v", rows)
		}
		holder.ExecContext(ctx, `ROLLBACK`)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		expect(t, s, &r, engine.StatusOK, "")
	})

	t.Run("MSSQL-TOPQUERY-001", func(t *testing.T) {
		r := *rs["MSSQL-TOPQUERY-001"]
		r.Params = map[string]float64{"dominant_pct": 0, "min_total_cpu_s": 0}
		expect(t, s, &r, engine.StatusFinding, engine.Hint)
		r.Params = map[string]float64{"dominant_pct": 101, "min_total_cpu_s": 0}
		expect(t, s, &r, engine.StatusOK, "")
	})

	t.Run("MSSQL-MISSINGIDX-001", func(t *testing.T) {
		r := *rs["MSSQL-MISSINGIDX-001"]
		r.Params = map[string]float64{"min_improvement": 0, "min_impact_pct": 0}
		exec(t, `IF OBJECT_ID('lens_fixture.dbo.mi') IS NOT NULL EXEC ('USE lens_fixture; DROP TABLE dbo.mi');
			EXEC ('USE lens_fixture; CREATE TABLE dbo.mi (id int IDENTITY PRIMARY KEY, k int, pad char(200));
			INSERT dbo.mi (k, pad) SELECT TOP (50000) ABS(CHECKSUM(NEWID())) % 1000, ''x'' FROM sys.all_objects a CROSS JOIN sys.all_objects b;')`)
		for i := 0; i < 5; i++ {
			exec(t, `EXEC ('USE lens_fixture; SELECT a.pad FROM dbo.mi a JOIN dbo.mi b ON b.id = a.id WHERE a.k = 42')`) // a join avoids a trivial plan
		}
		if !mentions(rowsOf(t, &r), "[mi]") {
			t.Error("missing index on lens_fixture.dbo.mi not reported")
		}
		exec(t, `EXEC ('USE lens_fixture; CREATE INDEX ix_k ON dbo.mi (k) INCLUDE (pad)')`)
		if mentions(rowsOf(t, &r), "[mi]") {
			t.Error("missing index still reported after creating it")
		}
	})

	t.Run("MSSQL-UNUSEDIDX-001", func(t *testing.T) {
		r := *rs["MSSQL-UNUSEDIDX-001"]
		r.Params = map[string]float64{"min_writes": 1}
		exec(t, `IF OBJECT_ID('lens_fixture.dbo.ui') IS NOT NULL EXEC ('USE lens_fixture; DROP TABLE dbo.ui');
			EXEC ('USE lens_fixture; CREATE TABLE dbo.ui (id int PRIMARY KEY, a int); CREATE INDEX ix_a ON dbo.ui (a);
			CREATE INDEX ix_a_dup ON dbo.ui (a); INSERT dbo.ui VALUES (1, 1), (2, 2); UPDATE dbo.ui SET a = a + 1;')`)
		rows := rowsOf(t, &r)
		if !mentions(rows, "ix_a") || !mentions(rows, "duplicate") {
			t.Errorf("unused/duplicate index not reported: %v", rows)
		}
		exec(t, `EXEC ('USE lens_fixture; DROP TABLE dbo.ui')`)
		if mentions(rowsOf(t, &r), "dbo.ui") {
			t.Error("dropped indexes still reported")
		}
	})

	t.Run("MSSQL-VERSION-001", func(t *testing.T) {
		r := *rs["MSSQL-VERSION-001"]
		if s.Info().Major > 16 {
			t.Skip("no lifecycle date yet for major", s.Info().Major)
		}
		r.Params = map[string]float64{"warn_days": 0} // supported today -> OK
		expect(t, s, &r, engine.StatusOK, "")
		r.Params = map[string]float64{"warn_days": 100000}
		expect(t, s, &r, engine.StatusFinding, engine.Hint)
	})
}

func TestPG(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, Conn{Engine: "pg", DSN: env("LENS_PG_DSN", "postgres://postgres:lens_dev@localhost:15417/postgres?sslmode=disable")})
	if err != nil {
		unreachable(t, "PostgreSQL", err)
	}
	defer s.Close()
	rs := catalog(t, "pg")

	t.Run("PG-VERSION-001", func(t *testing.T) {
		r := *rs["PG-VERSION-001"]
		r.Params = map[string]float64{"warn_days": 0} // every matrix version is supported today -> OK
		expect(t, s, &r, engine.StatusOK, "")
		r.Params = map[string]float64{"warn_days": 100000}
		expect(t, s, &r, engine.StatusFinding, "")
	})

	t.Run("PG-WRAPAROUND-001", func(t *testing.T) {
		r := *rs["PG-WRAPAROUND-001"]
		expect(t, s, &r, engine.StatusOK, "")
		r.Params = map[string]float64{"hint_ratio": 0, "confirmed_xid_age": 1e12}
		expect(t, s, &r, engine.StatusFinding, engine.Hint)
		r.Params = map[string]float64{"hint_ratio": 0, "confirmed_xid_age": 0}
		expect(t, s, &r, engine.StatusFinding, engine.Confirmed)
	})

	// admin is a separate, writable superuser connection used only to build fixtures.
	dsn := env("LENS_PG_DSN", "postgres://postgres:lens_dev@localhost:15417/postgres?sslmode=disable")
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	pexec := func(t *testing.T, q string) {
		t.Helper()
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}

	t.Run("PG-PGSS-001", func(t *testing.T) {
		pexec(t, `DROP EXTENSION IF EXISTS pg_stat_statements`)
		expect(t, s, rs["PG-PGSS-001"], engine.StatusFinding, engine.Confirmed)
		pexec(t, `CREATE EXTENSION pg_stat_statements`) // the containers preload it
		expect(t, s, rs["PG-PGSS-001"], engine.StatusOK, "")
	})

	t.Run("PG-CONN-001", func(t *testing.T) {
		r := *rs["PG-CONN-001"]
		expect(t, s, &r, engine.StatusOK, "")
		r.Params = map[string]float64{"hint_pct": 0, "confirmed_pct": 1000, "idle_min": 0, "idle_pct": 0}
		expect(t, s, &r, engine.StatusFinding, engine.Hint)
	})

	t.Run("PG-SLOTS-001", func(t *testing.T) {
		r := *rs["PG-SLOTS-001"]
		pexec(t, `SELECT pg_drop_replication_slot(slot_name) FROM pg_replication_slots WHERE slot_name = 'lens_slot'`)
		r.Params = map[string]float64{"retained_mb": 0, "confirmed_retained_mb": 1e9, "lag_s": 1e9}
		expect(t, s, &r, engine.StatusOK, "")
		pexec(t, `SELECT pg_create_physical_replication_slot('lens_slot', true)`) // inactive, WAL reserved
		expect(t, s, &r, engine.StatusFinding, engine.Hint)
		pexec(t, `SELECT pg_drop_replication_slot('lens_slot')`)
		expect(t, s, rs["PG-SLOTS-001"], engine.StatusOK, "")
	})

	t.Run("PG-SIZE-001", func(t *testing.T) {
		r := *rs["PG-SIZE-001"]
		expect(t, s, &r, engine.StatusOK, "")
		pexec(t, `CREATE TABLE IF NOT EXISTS lens_big AS SELECT g AS id, repeat('x', 200) AS pad FROM generate_series(1, 20000) g`)
		r.Params = map[string]float64{"min_rel_gb": 0}
		res := engine.Run(ctx, s, []*engine.Rule{&r}, nil)[0]
		found := false
		for _, row := range res.Rows {
			found = found || row[1] == "public.lens_big"
		}
		if res.Status != engine.StatusFinding || res.Columns[0] != "database" || !found {
			t.Errorf("lens_big not reported per database: %s %v %v", res.Reason, res.Columns, res.Rows)
		}
	})

	t.Run("PG-LONGTX-001", func(t *testing.T) {
		r := *rs["PG-LONGTX-001"]
		r.Params = map[string]float64{"max_tx_s": 1e9, "idle_tx_s": 0}
		expect(t, s, &r, engine.StatusOK, "")
		holder, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close(ctx)
		if _, err := holder.Exec(ctx, `BEGIN; SELECT 1;`); err != nil {
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond)
		expect(t, s, &r, engine.StatusFinding, engine.Confirmed)
		holder.Exec(ctx, `COMMIT`)
		expect(t, s, &r, engine.StatusOK, "")
	})

	// rowsWith returns whether any evidence row of a rule mentions s (in any column).
	rowsWith := func(t *testing.T, r *engine.Rule, s2 string) bool {
		t.Helper()
		res := engine.Run(ctx, s, []*engine.Rule{r}, nil)[0]
		if res.Status == engine.StatusNoData {
			t.Fatal(r.ID, res.Reason)
		}
		for _, row := range res.Rows {
			for _, v := range row {
				if str, ok := v.(string); ok && strings.Contains(str, s2) {
					return true
				}
			}
		}
		return false
	}

	t.Run("PG-LOCKS-001", func(t *testing.T) {
		r := *rs["PG-LOCKS-001"]
		r.Params = map[string]float64{"min_wait_s": 0}
		expect(t, s, &r, engine.StatusOK, "")
		pexec(t, `CREATE TABLE IF NOT EXISTS lens_lock (v int)`)
		holder, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close(ctx)
		if _, err := holder.Exec(ctx, `BEGIN; LOCK TABLE lens_lock IN ACCESS EXCLUSIVE MODE;`); err != nil {
			t.Fatal(err)
		}
		waiter, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer waiter.Close(ctx)
		done := make(chan error, 1)
		go func() { _, err := waiter.Exec(ctx, `SELECT count(*) FROM lens_lock`); done <- err }()
		time.Sleep(700 * time.Millisecond)
		if !rowsWith(t, &r, "idle in transaction") {
			t.Error("lock wait not reported with the idle-in-transaction blocker")
		}
		holder.Exec(ctx, `ROLLBACK`)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		expect(t, s, &r, engine.StatusOK, "")
	})

	t.Run("PG-TOPQUERY-001", func(t *testing.T) {
		pexec(t, `CREATE EXTENSION IF NOT EXISTS pg_stat_statements`)
		pexec(t, `SELECT count(*) FROM generate_series(1, 200000)`)
		r := *rs["PG-TOPQUERY-001"]
		r.Params = map[string]float64{"dominant_pct": 0, "min_total_s": 0}
		expect(t, s, &r, engine.StatusFinding, engine.Hint)
		r.Params = map[string]float64{"dominant_pct": 101, "min_total_s": 0}
		expect(t, s, &r, engine.StatusOK, "")
	})

	t.Run("PG-AUTOVAC-001", func(t *testing.T) {
		r := *rs["PG-AUTOVAC-001"]
		r.Params = map[string]float64{"min_dead": 1, "dead_pct": 1, "confirmed_dead_pct": 50}
		pexec(t, `DROP TABLE IF EXISTS lens_dead`)
		pexec(t, `CREATE TABLE lens_dead WITH (autovacuum_enabled = false) AS SELECT g AS id FROM generate_series(1, 20000) g`)
		pexec(t, `DELETE FROM lens_dead WHERE id <= 15000`)
		found := false
		for i := 0; i < 20 && !found; i++ { // table statistics are flushed asynchronously
			pexec(t, `SELECT pg_sleep(0.3)`)
			found = rowsWith(t, &r, "public.lens_dead")
		}
		if !found {
			t.Error("dead rows on lens_dead not reported")
		}
		pexec(t, `VACUUM lens_dead`)
		cleared := false
		for i := 0; i < 20 && !cleared; i++ {
			pexec(t, `SELECT pg_sleep(0.3)`)
			cleared = !rowsWith(t, &r, "public.lens_dead")
		}
		if !cleared {
			t.Error("lens_dead still reported after VACUUM")
		}
	})

	t.Run("PG-INDEXES-001", func(t *testing.T) {
		r := *rs["PG-INDEXES-001"]
		r.Params = map[string]float64{"min_unused_mb": 0}
		pexec(t, `DROP TABLE IF EXISTS lens_idx`)
		pexec(t, `CREATE TABLE lens_idx AS SELECT g % 10 AS v FROM generate_series(1, 1000) g`)
		pexec(t, `CREATE INDEX lens_idx_a ON lens_idx (v)`)
		pexec(t, `CREATE INDEX lens_idx_b ON lens_idx (v)`)
		// a failed CREATE INDEX CONCURRENTLY leaves an invalid index behind
		if _, err := admin.Exec(ctx, `CREATE UNIQUE INDEX CONCURRENTLY lens_idx_bad ON lens_idx (v)`); err == nil {
			t.Fatal("expected the unique index build to fail")
		}
		for _, want := range []string{"lens_idx_bad", "duplicate", "unused"} {
			if !rowsWith(t, &r, want) {
				t.Errorf("%q not reported", want)
			}
		}
		pexec(t, `DROP TABLE lens_idx`)
		if rowsWith(t, &r, "lens_idx") {
			t.Error("dropped table's indexes still reported")
		}
	})

	t.Run("PG-BLOAT-001", func(t *testing.T) {
		r := *rs["PG-BLOAT-001"]
		r.Params = map[string]float64{"min_size_mb": 0, "bloat_pct": 30}
		pexec(t, `DROP TABLE IF EXISTS lens_bloat`)
		pexec(t, `CREATE TABLE lens_bloat WITH (autovacuum_enabled = false) AS SELECT g AS id, repeat('x', 100) AS pad FROM generate_series(1, 50000) g`)
		pexec(t, `UPDATE lens_bloat SET id = id + 1`)
		pexec(t, `UPDATE lens_bloat SET id = id + 1`)
		pexec(t, `VACUUM ANALYZE lens_bloat`) // frees space inside the file but keeps the pages
		if !rowsWith(t, &r, "public.lens_bloat") {
			t.Error("bloat on lens_bloat not estimated")
		}
		pexec(t, `VACUUM FULL lens_bloat`)
		pexec(t, `ANALYZE lens_bloat`)
		if rowsWith(t, &r, "public.lens_bloat") {
			t.Error("lens_bloat still reported after VACUUM FULL")
		}
		pexec(t, `DROP TABLE lens_bloat`)
	})

	t.Run("read-only session", func(t *testing.T) {
		if _, _, err := s.Query(ctx, `CREATE TABLE lens_must_fail (id int)`); err == nil {
			t.Fatal("session accepted DDL; default_transaction_read_only is not in effect")
		}
	})
}
