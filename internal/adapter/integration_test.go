//go:build integration

// Runs the real catalog against the docker-compose databases (or LENS_MSSQL_SERVER / LENS_PG_DSN in CI).
// Each rule is provoked (finding expected) and checked clean (OK expected).
package adapter

import (
	"context"
	"os"
	"testing"

	"github.com/codedb-monitorsql/codedb-lens/internal/engine"
	"github.com/codedb-monitorsql/codedb-lens/rules"
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

	t.Run("read-only session", func(t *testing.T) {
		if _, _, err := s.Query(ctx, `CREATE TABLE lens_must_fail (id int)`); err == nil {
			t.Fatal("session accepted DDL; default_transaction_read_only is not in effect")
		}
	})
}
