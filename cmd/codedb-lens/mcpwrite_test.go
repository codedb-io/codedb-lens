package main

import (
	"os"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/profile"
)

// withStdin runs f with typed as the user's input.
func withStdin(t *testing.T, typed string, f func() error) error {
	r, w, _ := os.Pipe()
	_, _ = w.WriteString(typed)
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	return f()
}

// Q44: the profile key is set only after the user types the profile name; revoke-write clears it.
func TestMCPAllowWrite(t *testing.T) {
	dir := t.TempDir()
	for _, k := range []string{"APPDATA", "XDG_CONFIG_HOME", "USERPROFILE", "HOME"} {
		t.Setenv(k, dir)
	}
	keyring.MockInit()
	f := profile.File{}
	f.Put(profile.Profile{Name: "App-Write", Conn: adapter.Conn{Engine: "pg", Server: "db"}})
	f.Put(profile.Profile{Name: "diag", Conn: adapter.Conn{Engine: "pg", Server: "db"}})
	_ = f.Save()
	key := func() bool { f, _ := profile.Load(); p, _ := f.Get("App-Write"); return p.MCPWrite }
	allow := func(typed string) error {
		return withStdin(t, typed, func() error { return mcpWriteCmd("allow-write", []string{"--profile", "app-write"}) })
	}

	old := stdinTerminal
	defer func() { stdinTerminal = old }()
	stdinTerminal = func() bool { return false } // echo App-Write | codedb-lens mcp allow-write ...
	if err := allow("App-Write\n"); err == nil || key() {
		t.Fatalf("piped name: key set (%v)", err)
	}
	stdinTerminal = func() bool { return true }
	for _, wrong := range []string{"\n", "app-write\n", "diag\n", "yes\n"} {
		if err := allow(wrong); err == nil || key() {
			t.Fatalf("typed %q: key set (%v)", wrong, err)
		}
	}
	if err := allow("App-Write\r\n"); err != nil || !key() {
		t.Fatalf("typed the name: %v, key %v", err, key())
	}
	if f, _ := profile.Load(); f.Last != "diag" {
		t.Errorf("allow-write changed the last used profile to %q", f.Last)
	}
	if err := mcpWriteCmd("revoke-write", []string{"--profile", "App-Write"}); err != nil || key() {
		t.Fatalf("revoke-write: %v, key %v", err, key())
	}
	if err := mcpWriteCmd("revoke-write", []string{"--profile", "nope"}); err == nil {
		t.Fatal("unknown profile accepted")
	}
}

// The write mode is told only in the docs (plan §7.8): not in the READMEs, not in the app.
func TestWriteModeOnlyInDocs(t *testing.T) {
	for _, f := range []string{"../../README.md", "../../README.pt-BR.md", "frontend/index.html"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "allow-write") || strings.Contains(string(raw), "execute_sql") {
			t.Errorf("%s mentions the write mode", f)
		}
	}
}
