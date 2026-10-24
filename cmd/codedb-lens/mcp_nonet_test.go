package main

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/mcpserver"
	"github.com/codedb-io/codedb-lens/internal/profile"
)

type countingTransport struct{ n atomic.Int32 }

func (c *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.n.Add(1)
	return nil, http.ErrUseLastResponse
}

// The SDK links net/http, so "no network" is proven at run time: a whole MCP session with main's real
// diagnosis, history and compare makes no HTTP call (no update check, no license call).
func TestMCPNoHTTP(t *testing.T) {
	dir := t.TempDir()
	for _, k := range []string{"APPDATA", "XDG_CONFIG_HOME", "USERPROFILE", "HOME"} {
		t.Setenv(k, dir)
	}
	keyring.MockInit() // never the user's real key or passwords
	f := profile.File{}
	f.Put(profile.Profile{Name: "local", Conn: adapter.Conn{Engine: "pg", Server: "127.0.0.1", Port: 1, User: "u"}})
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	_ = profile.SetPassword("local", "pw")

	ct := &countingTransport{}
	old := http.DefaultTransport
	http.DefaultTransport = ct
	t.Cleanup(func() { http.DefaultTransport = old })

	srv, err := mcpserver.New(mcpOptions("en", "", false))
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
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"profile": "local", "rule_id": "PG-LOCKS-001", "folder": dir, "duration": "1h"}
	for _, tool := range tools.Tools {
		if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool.Name, Arguments: args}); err != nil {
			t.Errorf("%s: %v", tool.Name, err) // tool errors (no server, no key) are fine; protocol errors are not
		}
	}
	if n := ct.n.Load(); n != 0 {
		t.Fatalf("%d HTTP call(s) during an MCP session", n)
	}
}
