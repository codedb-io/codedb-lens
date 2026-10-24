package main

import (
	"testing"

	"github.com/codedb-io/codedb-lens/internal/mcpserver"
)

// The GUI shows what mcp install writes: same generator, same arguments (plan §7.3.2).
func TestMCPGuides(t *testing.T) {
	exe := `C:\Users\Maria Silva\AppData\Local\Programs\codeDB Lens\codedb-lens.exe`
	profiles := []string{"prod", "dev"}
	args := mcpserver.Args(profiles, true)
	gs := mcpGuides(exe, profiles, true)
	if len(gs) != len(mcpserver.Clients) {
		t.Fatalf("%d guides for %d clients", len(gs), len(mcpserver.Clients))
	}
	for _, g := range gs {
		c, _ := mcpserver.FindClient(g.Client)
		if g.Client == "claude-code" {
			if g.Command != mcpserver.ClaudeCodeCommand(exe, args) || g.Snippet != "" {
				t.Errorf("claude-code: %+v", g)
			}
			continue
		}
		if want := `"` + exe + `" mcp install --client ` + g.Client + ` --profile prod,dev --redact`; g.Command != want {
			t.Errorf("%s command:\n%s\n%s", g.Client, g.Command, want)
		}
		if g.Snippet != c.Snippet(exe, args) {
			t.Errorf("%s snippet differs from mcp install's", g.Client)
		}
	}
}
