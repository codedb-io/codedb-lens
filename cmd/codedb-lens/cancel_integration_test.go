//go:build integration

// The GUI's "Stop" cancels the Diagnose context. Against the docker-compose databases, check that
// the query in flight dies, no report is produced, and no "codeDB Lens" session is left on the server.
package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/microsoft/go-mssqldb"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/engine"
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func checkCancel(t *testing.T, c adapter.Conn, slow string, lensSessions func() int) {
	// 1. A diagnosis cancelled mid-catalog returns context.Canceled, without a report.
	ctx, cancel := context.WithCancel(context.Background())
	rep, err := Diagnose(ctx, c, "en", "", func(string) {}, func(i, _ int, _ *engine.Rule, res *engine.Result) {
		if i == 2 && res == nil {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || rep != nil {
		t.Fatalf("Diagnose after cancel: rep=%v err=%v", rep != nil, err)
	}

	// 2. A query in flight is interrupted at once, not after it finishes.
	s, err := adapter.Open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	qctx, qcancel := context.WithCancel(context.Background())
	time.AfterFunc(500*time.Millisecond, qcancel)
	start := time.Now()
	_, _, err = s.Query(qctx, slow)
	if took := time.Since(start); err == nil || took > 3*time.Second {
		t.Fatalf("slow query not interrupted: took %s, err %v", took, err)
	}
	s.Close()

	// 3. Nothing of ours is left connected.
	deadline := time.Now().Add(5 * time.Second)
	for n := lensSessions(); n != 0; n = lensSessions() {
		if time.Now().After(deadline) {
			t.Fatalf("%d %q session(s) still open after stop", n, adapter.AppName)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestStopMSSQL(t *testing.T) {
	server, pw := envOr("LENS_MSSQL_SERVER", "localhost,14322"), envOr("MSSQL_SA_PASSWORD", "Lens_dev_Passw0rd!")
	admin, _ := sql.Open("sqlserver", "sqlserver://sa:"+pw+"@localhost:14322?encrypt=true&TrustServerCertificate=true")
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skip("SQL Server not reachable:", err)
	}
	checkCancel(t, adapter.Conn{Engine: "mssql", Server: server, User: "sa", Password: pw, TrustServerCert: true},
		"WAITFOR DELAY '00:00:08'", func() (n int) {
			_ = admin.QueryRow("SELECT COUNT(*) FROM sys.dm_exec_sessions WHERE program_name = @p1", adapter.AppName).Scan(&n)
			return n
		})
}

func TestStopPG(t *testing.T) {
	dsn := envOr("LENS_PG_DSN", "postgres://postgres:lens_dev@localhost:15417/postgres?sslmode=disable")
	admin, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Skip("PostgreSQL not reachable:", err)
	}
	defer admin.Close(context.Background())
	checkCancel(t, adapter.Conn{Engine: "pg", DSN: dsn}, "SELECT pg_sleep(8)", func() (n int) {
		_ = admin.QueryRow(context.Background(), "SELECT count(*) FROM pg_stat_activity WHERE application_name = $1", adapter.AppName).Scan(&n)
		return n
	})
}
