//go:build integration

// The MCP check against the docker-compose databases: the same diagnosis as the CLI, a report in the
// history, the evidence through get_finding, and a client cancel that leaves no session behind.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/mcpserver"
	"github.com/codedb-io/codedb-lens/internal/profile"
)

func mcpCheck(t *testing.T, c adapter.Conn, lensSessions func() int) {
	dir := t.TempDir()
	for _, k := range []string{"APPDATA", "XDG_CONFIG_HOME", "USERPROFILE", "HOME"} {
		t.Setenv(k, dir)
	}
	keyring.MockInit()
	o := mcpOptions("en", "", false)
	o.Fixed = &profile.Profile{Name: "default", Conn: c}
	srv, err := mcpserver.New(o)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, cl := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, cl, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	call := func(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			return nil, err
		}
		if res.IsError {
			t.Fatalf("%s: %s", name, res.Content[0].(*mcp.TextContent).Text)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var m map[string]any
		return m, json.Unmarshal(raw, &m)
	}

	m, err := call(ctx, "check", map[string]any{"profile": "default"})
	if err != nil {
		t.Fatal(err)
	}
	if m["coverage"].(float64) == 0 || m["server"].(map[string]any)["engine"] != c.Engine {
		t.Fatalf("check: %v", m)
	}
	if _, err := os.Stat(m["report"].(string)); err != nil {
		t.Fatalf("report not saved: %v", err)
	}
	rule := ""
	for _, k := range []string{"findings", "no_data"} {
		if l := m[k].([]any); len(l) > 0 && rule == "" {
			rule = l[0].(map[string]any)["id"].(string)
		}
	}
	if rule != "" {
		if f, err := call(ctx, "get_finding", map[string]any{"profile": "default", "rule_id": rule}); err != nil || f["id"] != rule {
			t.Fatalf("get_finding %s: %v %v", rule, f, err)
		}
	}
	if h, _ := call(ctx, "history", map[string]any{"profile": "default"}); len(h["runs"].([]any)) != 1 {
		t.Fatalf("history: %v", h)
	}

	// The client gives up mid-check (propagation is unit-tested in mcpserver): nothing of ours stays connected.
	cctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_, _ = call(cctx, "check", map[string]any{"profile": "default"})
	deadline := time.Now().Add(5 * time.Second)
	for n := lensSessions(); n != 0; n = lensSessions() {
		if time.Now().After(deadline) {
			t.Fatalf("%d %q session(s) still open after the client cancelled", n, adapter.AppName)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// skipOrFail skips on a laptop without the databases; in CI (LENS_REQUIRE_DB) a missing database is a failure.
func skipOrFail(t *testing.T, args ...any) {
	if os.Getenv("LENS_REQUIRE_DB") != "" {
		t.Fatal(args...)
	}
	t.Skip(args...)
}

func TestMSSQLMCPCheck(t *testing.T) {
	server, pw := envOr("LENS_MSSQL_SERVER", "localhost,14322"), envOr("MSSQL_SA_PASSWORD", "Lens_dev_Passw0rd!")
	admin, _ := sql.Open("sqlserver", "sqlserver://sa:"+pw+"@"+strings.Replace(server, ",", ":", 1)+"?encrypt=true&TrustServerCertificate=true")
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		skipOrFail(t, "SQL Server not reachable:", err)
	}
	mcpCheck(t, adapter.Conn{Engine: "mssql", Server: server, User: "sa", Password: pw, TrustServerCert: true}, func() (n int) {
		_ = admin.QueryRow("SELECT COUNT(*) FROM sys.dm_exec_sessions WHERE program_name = @p1 AND host_process_id = @p2",
			adapter.AppName, os.Getpid()).Scan(&n)
		return n
	})
}

func TestPGMCPCheck(t *testing.T) {
	dsn := envOr("LENS_PG_DSN", "postgres://postgres:lens_dev@localhost:15417/postgres?sslmode=disable")
	admin, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		skipOrFail(t, "PostgreSQL not reachable:", err)
	}
	defer admin.Close(context.Background())
	mcpCheck(t, adapter.Conn{Engine: "pg", DSN: dsn}, func() (n int) {
		_ = admin.QueryRow(context.Background(), "SELECT count(*) FROM pg_stat_activity WHERE application_name = $1", adapter.AppName).Scan(&n)
		return n
	})
}
