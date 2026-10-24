package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/internal/record"
	"github.com/codedb-io/codedb-lens/internal/report"
	"github.com/codedb-io/codedb-lens/rules"
)

type checkCall struct {
	conn   adapter.Conn
	redact bool
}

// fakeCheck stands in for main's Diagnose + saveReport: one finding with 7 evidence rows naming
// the database SalesDB, one rule without data, one OK; saved in the profile's folder like the real one.
func fakeCheck(t *testing.T, calls *[]checkCall) func(context.Context, profile.Profile, adapter.Conn, string, bool) (*report.Report, string, error) {
	cat, err := engine.LoadCatalog(rules.FS, "mssql")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*engine.Rule{}
	for _, r := range cat {
		byID[r.ID] = r
	}
	n := 0
	return func(_ context.Context, p profile.Profile, c adapter.Conn, lang string, redact bool) (*report.Report, string, error) {
		if calls != nil {
			*calls = append(*calls, checkCall{c, redact})
		}
		var rows [][]any
		for i := range 7 {
			rows = append(rows, []any{"SalesDB", i})
		}
		rep := report.New("test", lang, engine.ServerInfo{Engine: "mssql", Version: "16.0", Platform: "onprem", Host: "sql01"},
			engine.Score{Value: 71, Letter: "C", Coverage: 0.9}, []engine.Result{
				{Rule: byID["MSSQL-BACKUP-001"], Status: engine.StatusFinding, Confidence: engine.Confirmed, Columns: []string{"database", "days"}, Rows: rows},
				{Rule: byID["MSSQL-AGENT-001"], Status: engine.StatusNoData, Reason: "no access to msdb"},
				{Rule: byID["MSSQL-CONFIG-001"], Status: engine.StatusOK},
			})
		n++
		rep.GeneratedAt = rep.GeneratedAt.Add(time.Duration(n) * time.Second) // file names have seconds
		if redact {
			salt, err := profile.RedactSalt()
			if err != nil {
				return nil, "", err
			}
			rep.Redact(salt)
		}
		path, err := rep.Save(reportsDir(p))
		return rep, path, err
	}
}

func TestCheckHistoryFinding(t *testing.T) {
	var calls []checkCall
	cs := connect(t, Options{Check: fakeCheck(t, &calls)})
	m, res := call(t, cs, "check", map[string]any{"profile": "prod"})
	if res.IsError {
		t.Fatalf("check: %v", res.Content)
	}
	if calls[0].conn.Password != sentinel || calls[0].redact {
		t.Fatalf("check did not get the stored password, or redacted unasked: %+v", calls[0])
	}
	if m["grade"] != "C" || m["passed"].(float64) != 1 || len(m["no_data"].([]any)) != 1 {
		t.Fatalf("check: %v", m)
	}
	f := m["findings"].([]any)[0].(map[string]any)
	if f["id"] != "MSSQL-BACKUP-001" || len(f["rows"].([]any)) != previewRows || f["total_rows"].(float64) != 7 {
		t.Fatalf("finding preview: %v", f)
	}
	if _, err := os.Stat(m["report"].(string)); err != nil {
		t.Fatalf("report not saved: %v", err)
	}
	if !strings.Contains(m["note"].(string), "utm_medium=mcp") {
		t.Errorf("CTA: %v", m["note"])
	}

	call(t, cs, "check", map[string]any{"profile": "prod", "redact": true})
	if !calls[1].redact {
		t.Error("redact argument ignored")
	}
	m, _ = call(t, cs, "history", map[string]any{"profile": "prod"})
	runs := m["runs"].([]any)
	if len(runs) != 2 || runs[0].(map[string]any)["grade"] != "C" {
		t.Fatalf("history: %v", m)
	}

	older := runs[1].(map[string]any)["run"].(string)
	m, _ = call(t, cs, "get_finding", map[string]any{"profile": "prod", "rule_id": "mssql-backup-001", "run": older})
	if len(m["rows"].([]any)) != 7 || m["status"] != "finding" || m["verify"] == "" || m["rows"].([]any)[0].([]any)[0] != "SalesDB" {
		t.Fatalf("get_finding (plain run): %v", m)
	}
	m, _ = call(t, cs, "get_finding", map[string]any{"profile": "prod", "rule_id": "MSSQL-BACKUP-001"})
	if m["rows"].([]any)[0].([]any)[0] == "SalesDB" {
		t.Fatal("latest run was redacted but get_finding shows the name")
	}

	// Paths outside the profile's folder are refused before anything is read.
	outside := filepath.Join(t.TempDir(), "2026-01-01_000000.json")
	for _, args := range []map[string]any{
		{"profile": "prod", "rule_id": "MSSQL-BACKUP-001", "run": outside},
		{"profile": "prod", "rule_id": "MSSQL-BACKUP-001", "run": filepath.Join(filepath.Dir(older), "..", "x.json")},
		{"profile": "prod", "rule_id": "NOPE-001"},
		{"profile": "pg-dev", "rule_id": "MSSQL-BACKUP-001"}, // no runs yet
	} {
		if _, res := call(t, cs, "get_finding", args); !res.IsError {
			t.Errorf("get_finding %v: want a tool error", args)
		}
	}
}

// A client cancel must reach the diagnosis context: that is what stops the query in flight (TestStopMSSQL/PG).
func TestCheckCancelledByClient(t *testing.T) {
	stopped := make(chan error, 1)
	cs := connect(t, Options{Check: func(ctx context.Context, _ profile.Profile, _ adapter.Conn, _ string, _ bool) (*report.Report, string, error) {
		<-ctx.Done()
		stopped <- ctx.Err()
		return nil, "", ctx.Err()
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "check", Arguments: map[string]any{"profile": "prod"}}); err == nil {
		t.Fatal("cancelled call: want an error")
	}
	select {
	case err := <-stopped:
		if err == nil {
			t.Fatal("diagnosis context not cancelled")
		}
	case <-time.After(time.Second):
		t.Fatal("the diagnosis kept running 1 s after the client cancelled")
	}
}

func TestCompareDefaultsToLatestTwo(t *testing.T) {
	var got []string
	cs := connect(t, Options{Compare: func(lang, a, b string) (*report.Comparison, string, error) {
		got = []string{a, b}
		ra, _ := report.Load(a)
		rb, _ := report.Load(b)
		c, err := report.Compare("test", lang, ra, rb, a, b)
		return c, "compare.html", err
	}})
	if _, res := call(t, cs, "compare", map[string]any{"profile": "prod"}); !res.IsError {
		t.Fatal("compare with no runs: want a tool error")
	}
	call(t, cs, "check", map[string]any{"profile": "prod"})
	call(t, cs, "check", map[string]any{"profile": "prod"})
	m, res := call(t, cs, "compare", map[string]any{"profile": "prod"})
	if res.IsError {
		t.Fatalf("compare: %v", res.Content)
	}
	if len(got) != 2 || got[0] >= got[1] || !strings.HasSuffix(got[0], ".json") {
		t.Fatalf("compare got %v, want the two latest JSON reports, older first", got)
	}
	if len(m["persisting"].([]any)) != 1 || m["after"].(map[string]any)["grade"] != "C" {
		t.Fatalf("compare: %v", m)
	}
}

func TestProfileAccessAndPassword(t *testing.T) {
	cs := connect(t, Options{Profiles: []string{"prod"}})
	for _, tool := range []string{"check", "history", "compare", "record_command", "list_recordings"} {
		_, res := call(t, cs, tool, map[string]any{"profile": "pg-dev"})
		if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "not available") {
			t.Errorf("%s on a hidden profile: want a tool error", tool)
		}
	}

	_ = keyring.Delete("codeDB Lens", "prod")
	t.Setenv("CODEDB_LENS_PASSWORD", "")
	if _, res := call(t, cs, "check", map[string]any{"profile": "prod"}); !res.IsError {
		t.Error("check without any password: want a tool error, never a prompt")
	}
	t.Setenv("CODEDB_LENS_PASSWORD", "from-env")
	if _, res := call(t, cs, "check", map[string]any{"profile": "prod"}); res.IsError {
		t.Errorf("check with CODEDB_LENS_PASSWORD: %v", res.Content)
	}
}

func TestFixedConnection(t *testing.T) {
	var calls []checkCall
	cs := connect(t, Options{Check: fakeCheck(t, &calls),
		Fixed: &profile.Profile{Name: "default", Conn: adapter.Conn{Engine: "pg", Server: "db", Password: "pw"}}})
	m, _ := call(t, cs, "list_profiles", map[string]any{})
	if ps := m["profiles"].([]any); len(ps) != 1 || ps[0].(map[string]any)["name"] != "default" {
		t.Fatalf("saved profiles must be hidden with a fixed connection: %v", ps)
	}
	call(t, cs, "check", map[string]any{"profile": "default"})
	if len(calls) != 1 || calls[0].conn.Password != "pw" {
		t.Fatalf("fixed connection: %+v", calls)
	}
	m, _ = call(t, cs, "record_command", map[string]any{"profile": "default"})
	if !strings.HasPrefix(m["command"].(string), "codedb-lens record --engine pg --server db") {
		t.Errorf("record_command without profiles: %v", m["command"])
	}
}

func TestRecordCommand(t *testing.T) {
	cs := connect(t, Options{})
	m, _ := call(t, cs, "record_command", map[string]any{"profile": "prod", "duration": "90m"})
	cmd := m["command"].(string)
	if !strings.HasPrefix(cmd, "codedb-lens record --profile prod --duration 1h30m --out ") || !strings.Contains(cmd, "recordings") {
		t.Errorf("command: %s", cmd)
	}
	if srv := m["server_command"].(string); srv != `codedb-lens record --engine mssql --server sql01,1433 --user lens --duration 1h30m` {
		t.Errorf("server command: %s", srv)
	}
	if !strings.Contains(m["note"].(string), "free key") {
		t.Errorf("more than 1 h must mention the free key: %s", m["note"])
	}
	m, _ = call(t, cs, "record_command", map[string]any{"profile": "prod"})
	if !strings.Contains(m["command"].(string), "--duration 1h ") || strings.Contains(m["note"].(string), "free key") {
		t.Errorf("default 1h: %v", m)
	}
	for _, d := range []string{"25h", "-1h", "soon"} {
		if _, res := call(t, cs, "record_command", map[string]any{"profile": "prod", "duration": d}); !res.IsError {
			t.Errorf("duration %q: want a tool error", d)
		}
	}
	if got := Quote(`C:\Users\Maria Silva\rec`); got != `"C:\Users\Maria Silva\rec"` {
		t.Errorf("quote: %s", got)
	}
}

var rec0 = time.Date(2026, 11, 2, 14, 0, 0, 0, time.UTC)

// fakeRecording writes 10 min of PostgreSQL lock samples into the pg-dev profile's recordings, with a
// 45 s lock wait at 14:02 (blocker user app_user) and a lost connection from 14:05 to 14:06.
func fakeRecording(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(recordingsDir(profile.Profile{Name: "pg-dev"}), "codedb-lens-record-db-20261102-1400")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := record.Meta{LensVersion: "0.9.0", StartedAt: rec0, Deadline: rec0.Add(time.Hour), StoppedAt: rec0.Add(10 * time.Minute),
		StopReason: "user", Intervals: map[string]string{"fast": "15s", "slow": "5m0s"},
		Server: engine.ServerInfo{Engine: "pg", Version: "17.2", Platform: "onprem", Host: "db01.acme.local"}}
	if err := record.WriteMeta(dir, meta); err != nil {
		t.Fatal(err)
	}
	w := record.NewWriter(dir)
	cols := []string{"blocked_pid", "blocked_by", "database", "waiting_s", "wait_event", "blocker_pid", "blocker_state",
		"blocker_user", "blocker_application", "blocker_transaction_s", "query_hash"}
	for i := range 40 {
		at := rec0.Add(time.Duration(i) * 15 * time.Second)
		if at.After(rec0.Add(5*time.Minute)) && at.Before(rec0.Add(6*time.Minute)) {
			_ = w.Write(record.Line{T: at, K: record.KindGap, Err: "connection refused"})
			continue
		}
		var locks [][]any
		if wait := at.Sub(rec0.Add(2 * time.Minute)).Seconds(); wait >= 0 && wait <= 45 {
			locks = [][]any{{int64(200), "{100}", "erp", wait + 1, "relation", int64(100), "idle in transaction", "app_user", "erp-api", wait + 3, "111"}}
		}
		_ = w.Write(record.Line{T: at, K: "locks", C: cols, R: locks})
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRecordings(t *testing.T) {
	cs := connect(t, Options{})
	dir := fakeRecording(t)
	m, _ := call(t, cs, "list_recordings", map[string]any{})
	list := m["recordings"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["folder"] != dir || list[0].(map[string]any)["profile"] != "pg-dev" {
		t.Fatalf("list_recordings: %v", m)
	}

	m, res := call(t, cs, "recording_report", map[string]any{"folder": dir})
	if res.IsError {
		t.Fatalf("recording_report: %v", res.Content)
	}
	if m["coverage"].(float64) >= 1 || len(m["gaps"].([]any)) != 1 {
		t.Errorf("coverage/gaps: %v %v", m["coverage"], m["gaps"])
	}
	f := m["findings"].([]any)
	if len(f) != 1 || f[0].(map[string]any)["id"] != "PG-REC-BLOCKING-001" {
		t.Fatalf("findings: %v", f)
	}
	if _, err := os.Stat(m["report"].(string)); err != nil {
		t.Errorf("report not written: %v", err)
	}

	m, _ = call(t, cs, "recording_report", map[string]any{"folder": dir,
		"from": "2026-11-02T14:04:00Z", "to": "2026-11-02T14:10:00Z"})
	if len(m["findings"].([]any)) != 0 {
		t.Errorf("the 14:02 episode is outside 14:04-14:10: %v", m["findings"])
	}
	for _, args := range []map[string]any{{"folder": dir, "from": "14:00"}, {"folder": t.TempDir()}} {
		if _, res := call(t, cs, "recording_report", args); !res.IsError {
			t.Errorf("recording_report %v: want a tool error", args)
		}
	}
}

func TestRedactFlag(t *testing.T) {
	var calls []checkCall
	cs := connect(t, Options{Redact: true, Check: fakeCheck(t, &calls)})
	call(t, cs, "check", map[string]any{"profile": "prod"})
	if !calls[0].redact {
		t.Fatal("--redact must force redaction in check")
	}
	m, _ := call(t, cs, "check", map[string]any{"profile": "prod"})
	if strings.Contains(m["server"].(map[string]any)["host"].(string), "sql01") {
		t.Error("host shown with --redact")
	}
	dir := fakeRecording(t)
	m, _ = call(t, cs, "list_recordings", map[string]any{})
	if m["recordings"].([]any)[0].(map[string]any)["host"] != nil {
		t.Errorf("list_recordings shows the host with --redact: %v", m)
	}
	m, _ = call(t, cs, "recording_report", map[string]any{"folder": dir})
	raw := strings.Join([]string{m["server"].(map[string]any)["host"].(string), m["report"].(string)}, " ")
	if strings.Contains(raw, "acme") || !strings.Contains(raw, ".redacted.html") {
		t.Errorf("recording_report with --redact: %s", raw)
	}
}

func TestPrompts(t *testing.T) {
	cs := connect(t, Options{})
	var names []string
	for p, err := range cs.Prompts(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, p.Name)
	}
	if strings.Join(names, ",") != "diagnose,explore_data,incident" {
		t.Fatalf("prompts: %v", names)
	}
	res, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "diagnose", Arguments: map[string]string{"profile": "prod"}})
	if err != nil || !strings.Contains(res.Messages[0].Content.(*mcp.TextContent).Text, `"prod"`) {
		t.Fatalf("diagnose prompt: %v %v", res, err)
	}
}
