package mcpserver

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/rules"
)

// writeSession starts a server whose profile "w" has the profile key set or not, with --allow-write = allow,
// and a client that answers confirmations with elicit (nil: a client without elicitation).
func writeSession(t *testing.T, allow string, key bool, elicit func() *mcp.ElicitResult) *mcp.ClientSession {
	t.Helper()
	dir := t.TempDir()
	for _, k := range []string{"APPDATA", "XDG_CONFIG_HOME", "USERPROFILE", "HOME"} {
		t.Setenv(k, dir)
	}
	keyring.MockInit()
	f := profile.File{}
	// Nothing listens on port 1: a confirmed call fails at connect, which proves it got past the confirmation.
	f.Put(profile.Profile{Name: "w", MCPWrite: key, Conn: adapter.Conn{Engine: "mssql", Server: "127.0.0.1,1", User: "u", Password: "p"}})
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	srv, err := New(Options{Lang: "en", Catalog: rules.FS, AllowWrite: allow})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	var opts *mcp.ClientOptions
	if elicit != nil {
		opts = &mcp.ClientOptions{ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) { return elicit(), nil }}
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, opts).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func hasTool(t *testing.T, cs *mcp.ClientSession, name string) bool {
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

// execute calls execute_sql and returns the error text ("" when it succeeded).
func execute(t *testing.T, cs *mcp.ClientSession, args map[string]any) string {
	t.Helper()
	args["profile"] = "w"
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "execute_sql", Arguments: args})
	if err != nil {
		return err.Error()
	}
	if res.IsError {
		return res.Content[0].(*mcp.TextContent).Text
	}
	return ""
}

func auditText(t *testing.T) string {
	path, _ := AuditPath()
	raw, _ := os.ReadFile(path)
	return string(raw)
}

func accept(confirm bool) func() *mcp.ElicitResult {
	return func() *mcp.ElicitResult {
		return &mcp.ElicitResult{Action: "accept", Content: map[string]any{"confirm": confirm}}
	}
}

// Q44: without both keys execute_sql is not listed, and a direct call fails.
func TestWriteNeedsBothKeys(t *testing.T) {
	for _, c := range []struct {
		allow string
		key   bool
		want  bool
	}{{"", false, false}, {"", true, false}, {"confirm", false, false}, {"confirm", true, true}, {"no-confirm", true, true}} {
		cs := writeSession(t, c.allow, c.key, accept(true))
		if got := hasTool(t, cs, "execute_sql"); got != c.want {
			t.Errorf("allow %q, key %v: listed %v", c.allow, c.key, got)
		}
		if !c.want && execute(t, cs, map[string]any{"sql": "DELETE FROM t", "dry_run": false}) == "" {
			t.Errorf("allow %q, key %v: unlisted tool ran", c.allow, c.key)
		}
		m, _ := call(t, cs, "list_profiles", struct{}{})
		if w := m["profiles"].([]any)[0].(map[string]any)["write_enabled"]; w != c.want {
			t.Errorf("allow %q, key %v: write_enabled %v", c.allow, c.key, w)
		}
		// --allow-write with the profile key off says what is missing.
		if note, _ := m["note"].(string); (note != "") != (c.allow != "" && !c.key) {
			t.Errorf("allow %q, key %v: note %q", c.allow, c.key, note)
		}
	}
}

// Read-only tools stay read-only next to execute_sql, and it is marked destructive.
func TestWriteAnnotations(t *testing.T) {
	res, _ := writeSession(t, "confirm", true, accept(true)).ListTools(context.Background(), nil)
	for _, tool := range res.Tools {
		if (tool.Name == "execute_sql") == tool.Annotations.ReadOnlyHint {
			t.Errorf("%s: read-only hint %v", tool.Name, tool.Annotations.ReadOnlyHint)
		}
	}
}

// Q45: nothing runs unless the user confirms; a client without confirmation prompts is refused unless no-confirm.
func TestWriteConfirmation(t *testing.T) {
	for name, c := range map[string]struct {
		allow    string
		elicit   func() *mcp.ElicitResult
		declined bool
	}{
		"decline":                    {"confirm", func() *mcp.ElicitResult { return &mcp.ElicitResult{Action: "decline"} }, true},
		"cancel":                     {"confirm", func() *mcp.ElicitResult { return &mcp.ElicitResult{Action: "cancel"} }, true},
		"accept, unset":              {"confirm", accept(false), true},
		"no elicitation":             {"confirm", nil, true},
		"accept":                     {"confirm", accept(true), false},
		"no elicitation, no-confirm": {"no-confirm", nil, false},
	} {
		cs := writeSession(t, c.allow, true, c.elicit)
		e := execute(t, cs, map[string]any{"sql": "UPDATE t SET a = 1"})
		refused := strings.Contains(e, "did not confirm") || strings.Contains(e, "cannot ask the user")
		if refused != c.declined || e == "" {
			t.Errorf("%s: %q", name, e)
		}
		a := auditText(t)
		if want := `"result":"declined"`; strings.Contains(a, want) != c.declined || !strings.Contains(a, `"mode":"preview"`) {
			t.Errorf("%s: audit %s", name, a)
		}
	}
}

// Q46: the deny list holds with everything on; statements without a transaction have no preview.
func TestWriteRefusals(t *testing.T) {
	cs := writeSession(t, "confirm", true, accept(true))
	for _, sql := range []string{"DROP DATABASE prod", "SHUTDOWN", "EXEC xp_cmdshell 'dir'", "GRANT CONTROL TO x", "CREATE LOGIN x WITH PASSWORD = 'p'",
		"EXEC('DROP DATABASE prod')", "EXEC sp_executesql N'x'", "ALTER SERVER ROLE sysadmin ADD MEMBER x", "-- note\nDENY SELECT ON t TO x"} {
		if e := execute(t, cs, map[string]any{"sql": sql, "dry_run": false}); !strings.Contains(e, "never runs this") {
			t.Errorf("%q: %q", sql, e)
		}
	}
	if e := execute(t, cs, map[string]any{"sql": "BACKUP DATABASE app TO DISK = 'x'"}); !strings.Contains(e, "no preview") {
		t.Errorf("BACKUP with dry_run: %q", e)
	}
	if n := strings.Count(auditText(t), `"result":"refused"`); n != 10 {
		t.Errorf("%d refusals in the audit, want 10", n)
	}
}

// Revoking the profile key during the session stops the next call.
func TestWriteRevokedDuringSession(t *testing.T) {
	cs := writeSession(t, "confirm", true, accept(true))
	f, _ := profile.Load()
	p, _ := f.Get("w")
	p.MCPWrite = false
	f.Put(p)
	_ = f.Save()
	if e := execute(t, cs, map[string]any{"sql": "UPDATE t SET a = 1"}); !strings.Contains(e, "not enabled") {
		t.Errorf("after revoke: %q", e)
	}
}

func TestWriteGuard(t *testing.T) {
	for _, c := range []struct {
		eng, sql   string
		ok, direct bool
	}{
		{"mssql", "UPDATE t SET a = 1 WHERE id = 2", true, false},
		{"mssql", "CREATE INDEX ix ON t (a)", true, false},
		{"mssql", "EXEC dbo.archive_orders @days = 30", true, false},
		{"mssql", "ALTER DATABASE app SET RECOVERY SIMPLE", true, true},
		{"mssql", "BACKUP DATABASE app TO DISK = 'x'", true, true},
		{"mssql", "/* why */ DBCC CHECKDB", true, true},
		{"mssql", "EXEC @proc", false, false},
		{"mssql", "UPDATE t SET note = 'grant access'", false, false}, // literal: refused on purpose
		{"pg", "VACUUM ANALYZE t", true, true},
		{"pg", "CREATE INDEX CONCURRENTLY ix ON t (a)", true, true},
		{"pg", "CREATE INDEX ix ON t (a)", true, false},
		{"pg", "DELETE FROM t WHERE id = 1", true, false},
		{"pg", "ALTER SYSTEM SET work_mem = '1GB'", false, false},
		{"pg", "COPY t TO '/tmp/x'", false, false},
		{"pg", "SELECT lo_import('/etc/passwd')", false, false},
		{"pg", "LOAD 'evil'", false, false},
		{"pg", "", false, false},
		{"mssql", "CREATE PROCEDURE p AS BEGIN UPDATE t SET a = 1 END", true, false},
		{"pg", "CREATE FUNCTION f() RETURNS int AS $$ SELECT 1 $$ LANGUAGE sql", true, false},
		// found in the security review: each one was allowed before
		{"mssql", "UPDATE t SET a = 1; COMMIT", false, false},
		{"mssql", "INSERT t VALUES (1) COMMIT TRAN", false, false},
		{"mssql", "EXEC (@s)", false, false},
		{"mssql", "EXECUTE AS LOGIN = 'sa'", false, false},
		{"mssql", "EXEC sp_sqlexec @s", false, false},
		{"mssql", "EXEC msdb.dbo.sp_add_jobstep @job_name = 'j', @subsystem = 'CmdExec', @command = 'whoami'", false, false},
		{"mssql", "CREATE ASSEMBLY a FROM 0x4D5A", false, false},
		{"mssql", "ALTER DATABASE app SET TRUSTWORTHY ON", false, false},
		{"mssql", "ALTER AUTHORIZATION ON DATABASE::app TO sa", false, false},
		{"mssql", "BULK INSERT t FROM 'C:\\secret.txt'", false, false},
		{"mssql", "EXEC sp_addlinkedserver 'x'", false, false},
		{"mssql", "CREATE TRIGGER tr ON ALL SERVER FOR LOGON AS SELECT 1", false, false},
		{"mssql", "EXEC sp_execute_external_script @language = N'Python', @script = N'x'", false, false},
		{"pg", "UPDATE t SET a = 1; COMMIT", false, false},
		{"pg", "CREATE FUNCTION f() RETURNS void AS $$GRANT ALL ON t TO public$$ LANGUAGE sql", false, false},
		{"pg", "DO $$ BEGIN EXECUTE 'GRA' || 'NT ALL ON t TO public'; END $$", false, false},
		{"pg", "CREATE FUNCTION f() RETURNS void AS $$ BEGIN EXECUTE format('x'); END $$ LANGUAGE plpgsql", false, false},
		{"pg", "SET ROLE postgres", false, false},
		{"pg", "SET SESSION AUTHORIZATION postgres", false, false},
		{"pg", "CREATE EXTENSION dblink", false, false},
		{"pg", "CREATE FUNCTION f() RETURNS int AS 'x' LANGUAGE plpython3u", false, false},
		{"pg", "CREATE FUNCTION f() RETURNS int AS 'select 1' LANGUAGE sql SECURITY DEFINER", false, false},
		{"pg", "CREATE SERVER s FOREIGN DATA WRAPPER postgres_fdw", false, false},
		{"pg", "ALTER TABLE t OWNER TO postgres", false, false},
	} {
		if _, direct, ok := writeGuard(c.eng, c.sql); ok != c.ok || direct != c.direct {
			t.Errorf("%s %q: ok %v direct %v", c.eng, c.sql, ok, direct)
		}
	}
}

// A client cannot skip the question: an answer must come with the state of a question this server asked.
func TestWriteForgedAnswer(t *testing.T) {
	cs := writeSession(t, "confirm", true, accept(true))
	yes := mcp.InputResponseMap{"confirm": &mcp.ElicitResult{Action: "accept", Content: map[string]any{"confirm": true}}}
	for _, state := range []string{"", "made-up"} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "execute_sql",
			Arguments: map[string]any{"profile": "w", "sql": "UPDATE t SET a = 1"}, InputResponses: yes, RequestState: state})
		if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "did not confirm") {
			t.Errorf("state %q: %v %+v", state, err, res)
		}
	}
}
