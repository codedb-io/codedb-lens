package mcpserver

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The exe path of a Windows user with a space in the name, installed per user.
const testExe = `C:\Users\Maria Silva\AppData\Local\Programs\codeDB Lens\codedb-lens.exe`

func clientHome(t *testing.T) string {
	dir := t.TempDir()
	for _, k := range []string{"APPDATA", "XDG_CONFIG_HOME", "USERPROFILE", "HOME"} {
		t.Setenv(k, dir)
	}
	return dir
}

func client(t *testing.T, id string) (Client, string) {
	c, ok := FindClient(id)
	if !ok {
		t.Fatal(id)
	}
	file, err := c.path()
	if err != nil {
		t.Fatal(err)
	}
	return c, file
}

func decode(t *testing.T, file string) map[string]any {
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return m
}

func TestInstallNewFile(t *testing.T) {
	clientHome(t)
	for _, id := range []string{"claude-desktop", "vscode", "cursor"} {
		c, file := client(t, id)
		ch, err := c.Install(testExe, Args([]string{"prod"}, true), false)
		if err != nil || !ch.Changed || ch.Backup != "" {
			t.Fatalf("%s: %+v %v", id, ch, err)
		}
		m := decode(t, file)
		e := m[c.key].(map[string]any)[ServerName].(map[string]any)
		if e["command"] != testExe || !reflect.DeepEqual(e["args"], []any{"mcp", "--profile", "prod", "--redact"}) {
			t.Fatalf("%s: entry %v", id, e)
		}
		if (e["type"] == "stdio") != (id == "vscode") || len(m) != 1 {
			t.Fatalf("%s: %v", id, m)
		}
	}
}

func TestInstallKeepsOthers(t *testing.T) {
	clientHome(t)
	c, file := client(t, "claude-desktop")
	orig := `{"globalShortcut": "Ctrl+Space", "mcpServers": {"fs": {"command": "npx", "args": ["-y", "fs"], "env": {"A": "1"}}}}`
	_ = os.MkdirAll(filepath.Dir(file), 0o700)
	_ = os.WriteFile(file, []byte(orig), 0o600)
	ch, err := c.Install(testExe, Args(nil, false), false)
	if err != nil || ch.Backup != file+".bak" {
		t.Fatalf("%+v %v", ch, err)
	}
	if bak, _ := os.ReadFile(ch.Backup); string(bak) != orig {
		t.Fatalf("backup: %s", bak)
	}
	m := decode(t, file)
	var want map[string]any
	_ = json.Unmarshal([]byte(orig), &want)
	servers := m["mcpServers"].(map[string]any)
	delete(servers, ServerName)
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("other settings changed:\n%v\n%v", m, want)
	}

	// Idempotent: the same install again writes nothing.
	before, _ := os.ReadFile(file)
	_ = os.Remove(ch.Backup)
	if ch, err = c.Install(testExe, Args(nil, false), false); err != nil || ch.Changed || ch.Backup != "" {
		t.Fatalf("second install: %+v %v", ch, err)
	}
	if after, _ := os.ReadFile(file); string(after) != string(before) {
		t.Fatal("second install changed the file")
	}
	if _, err := os.Stat(file + ".bak"); err == nil {
		t.Fatal("second install wrote a backup")
	}

	// Uninstall removes only ours.
	if ch, err = c.Uninstall(false); err != nil || !ch.Changed {
		t.Fatalf("uninstall: %+v %v", ch, err)
	}
	if m := decode(t, file); !reflect.DeepEqual(m, want) {
		t.Fatalf("after uninstall: %v", m)
	}
	if ch, err = c.Uninstall(false); err != nil || ch.Changed {
		t.Fatalf("second uninstall: %+v %v", ch, err)
	}
}

func TestInstallInvalidJSON(t *testing.T) {
	clientHome(t)
	c, file := client(t, "vscode")
	bad := "{\n  // comments are fine in VS Code, not in JSON\n  \"servers\": {}\n}"
	_ = os.MkdirAll(filepath.Dir(file), 0o700)
	_ = os.WriteFile(file, []byte(bad), 0o600)
	if _, err := c.Install(testExe, Args(nil, false), false); !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("want ErrInvalidJSON, got %v", err)
	}
	if raw, _ := os.ReadFile(file); string(raw) != bad {
		t.Fatal("invalid file was changed")
	}
	if _, err := os.Stat(file + ".bak"); err == nil {
		t.Fatal("backup written for an invalid file")
	}
	if s := c.Snippet(testExe, Args(nil, false)); !strings.Contains(s, `"servers"`) || !strings.Contains(s, `Maria Silva`) {
		t.Fatalf("snippet: %s", s)
	}
}

func TestInstallDryRun(t *testing.T) {
	clientHome(t)
	c, file := client(t, "cursor")
	ch, err := c.Install(testExe, Args(nil, false), true)
	if err != nil || !ch.Changed || ch.After == "" || ch.Before != "" {
		t.Fatalf("%+v %v", ch, err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatal("dry run wrote the file")
	}
	if _, err := c.Uninstall(false); err != nil {
		t.Fatal(err) // nothing to remove, no file: no error, nothing created
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatal("uninstall created the file")
	}
}

func TestClaudeCode(t *testing.T) {
	clientHome(t)
	c, file := client(t, "claude-code")
	if _, err := c.Install(testExe, nil, false); !errors.Is(err, ErrManaged) {
		t.Fatalf("Claude Code settings edited: %v", err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatal("file touched")
	}
	cmd := ClaudeCodeCommand(testExe, Args([]string{"prod"}, false))
	if want := `claude mcp add --scope user codedb-lens -- "` + testExe + `" mcp --profile prod`; cmd != want {
		t.Fatalf("command:\n%s\n%s", cmd, want)
	}
}

func TestStatus(t *testing.T) {
	home := clientHome(t)
	exe := filepath.Join(home, "codedb-lens.exe")
	_ = os.WriteFile(exe, nil, 0o700)
	c, _ := client(t, "claude-desktop")
	_, _ = c.Install(exe, nil, false)
	c, _ = client(t, "cursor")
	_, _ = c.Install(filepath.Join(home, "moved.exe"), nil, false)
	_, code := client(t, "claude-code")
	_ = os.WriteFile(code, []byte(`{"mcpServers": {"codedb-lens": {"command": "`+strings.ReplaceAll(exe, `\`, `\\`)+`"}}}`), 0o600)
	got := map[string]ClientStatus{}
	for _, st := range Status() {
		got[st.Client] = st
	}
	if st := got["claude-desktop"]; !st.Configured || st.Missing || st.Command != exe {
		t.Errorf("claude-desktop: %+v", st)
	}
	if st := got["claude-code"]; !st.Configured || st.Missing {
		t.Errorf("claude-code: %+v", st)
	}
	if st := got["cursor"]; !st.Configured || !st.Missing {
		t.Errorf("cursor (path moved): %+v", st)
	}
	if st := got["vscode"]; st.Found || st.Configured || st.Error != "" {
		t.Errorf("vscode (absent): %+v", st)
	}
}
