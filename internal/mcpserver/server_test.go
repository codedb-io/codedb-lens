package mcpserver

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/rules"
)

var update = flag.Bool("update", false, "rewrite testdata/tools.golden.json")

const sentinel = "PW_SENTINEL"

// connect runs the server against a real SDK client over in-memory pipes, with an isolated
// config folder holding two profiles whose passwords are the sentinel.
func connect(t *testing.T, o Options) *mcp.ClientSession {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)         // Windows
	t.Setenv("XDG_CONFIG_HOME", dir) // Linux
	keyring.MockInit()
	f := profile.File{}
	f.Put(profile.Profile{Name: "prod", Conn: adapter.Conn{Engine: "mssql", Server: "sql01,1433", User: "lens", Password: sentinel}})
	f.Put(profile.Profile{Name: "pg-dev", Lang: "pt-BR", Conn: adapter.Conn{Engine: "pg", Server: "db", Database: "app", DSN: "postgres://u:" + sentinel + "@db"}})
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"prod", "pg-dev"} {
		if err := profile.SetPassword(n, sentinel); err != nil {
			t.Fatal(err)
		}
	}

	if o.Catalog == nil {
		o.Catalog = rules.FS
	}
	if o.Lang == "" {
		o.Lang = "en"
	}
	srv, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

// call returns the structured result and fails on protocol errors; tool errors come back in isError.
func call(t *testing.T, cs *mcp.ClientSession, name string, args any) (map[string]any, *mcp.CallToolResult) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	raw, _ := json.Marshal(res)
	if strings.Contains(string(raw), sentinel) {
		t.Fatalf("%s leaked a password: %s", name, raw)
	}
	m, _ := res.StructuredContent.(map[string]any)
	return m, res
}

// The tool descriptions are the prompt the model reads: any change must show up in review.
func TestToolsGolden(t *testing.T) {
	cs := connect(t, Options{Version: "test"})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", tool.Name)
		}
	}
	got, _ := json.MarshalIndent(res.Tools, "", "  ")
	golden := filepath.Join("testdata", "tools.golden.json")
	if *update {
		_ = os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(golden, append(got, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/mcpserver -update)", err)
	}
	if strings.TrimSpace(strings.ReplaceAll(string(want), "\r\n", "\n")) != string(got) {
		t.Errorf("tools/list changed; review and run go test ./internal/mcpserver -update\n%s", got)
	}
}

func TestListProfiles(t *testing.T) {
	names := func(m map[string]any) []string {
		var out []string
		for _, p := range m["profiles"].([]any) {
			out = append(out, p.(map[string]any)["name"].(string))
		}
		return out
	}
	m, _ := call(t, connect(t, Options{}), "list_profiles", map[string]any{})
	if got := strings.Join(names(m), ","); got != "pg-dev,prod" {
		t.Fatalf("all profiles: %s", got)
	}
	p := m["profiles"].([]any)[1].(map[string]any)
	if p["engine"] != "mssql" || p["host"] != "sql01,1433" {
		t.Fatalf("profile fields: %v", p)
	}
	m, _ = call(t, connect(t, Options{Profiles: []string{"PROD"}}), "list_profiles", map[string]any{})
	if got := strings.Join(names(m), ","); got != "prod" {
		t.Fatalf("--profile PROD: %s", got)
	}
}

func TestRules(t *testing.T) {
	cs := connect(t, Options{Lang: "pt-BR"})
	m, _ := call(t, cs, "list_rules", map[string]any{"engine": "pg"})
	list := m["rules"].([]any)
	if len(list) < 10 {
		t.Fatalf("pg rules: %d", len(list))
	}
	series := false
	for _, r := range list {
		r := r.(map[string]any)
		if r["engine"] != "pg" || r["title"] == "" {
			t.Fatalf("rule: %v", r)
		}
		series = series || r["kind"] == "series"
	}
	if !series {
		t.Error("series rules missing from list_rules")
	}

	m, _ = call(t, cs, "explain_rule", map[string]any{"rule_id": "mssql-backup-001"})
	if m["id"] != "MSSQL-BACKUP-001" || m["sql"] == "" || !strings.HasPrefix(m["doc_url"].(string), "https://lens.codedb.com.br/rules/") {
		t.Fatalf("explain_rule: %v", m)
	}

	for name, args := range map[string]map[string]any{
		"explain_rule": {"rule_id": "NOPE-001"},
		"list_rules":   {"engine": "oracle"},
	} {
		_, res := call(t, cs, name, args)
		if !res.IsError {
			t.Errorf("%s %v: want a tool error", name, args)
		}
		if text := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(text, "não") {
			t.Errorf("%s: error not in pt-BR: %s", name, text)
		}
	}
}
