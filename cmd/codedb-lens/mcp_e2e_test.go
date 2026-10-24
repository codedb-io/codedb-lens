package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMCPStdio starts the real binary the way an AI client does (pipes, no console) and checks that
// stdout carries nothing but JSON-RPC. LENS_BIN tests a prebuilt binary instead, e.g. the
// GUI-subsystem Windows .exe in CI, where stdio over pipes is the main risk.
func TestMCPStdio(t *testing.T) {
	bin := os.Getenv("LENS_BIN")
	if bin == "" {
		bin = filepath.Join(t.TempDir(), "codedb-lens.exe")
		if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
	}
	dir := t.TempDir()
	cmd := exec.Command(bin, "mcp")
	cmd.Env = append(os.Environ(), "APPDATA="+dir, "XDG_CONFIG_HOME="+dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()

	for _, req := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"e2e","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_rules","arguments":{}}}`,
	} {
		io.WriteString(stdin, req+"\n")
	}

	type msg struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	got := map[int]msg{}
	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(nil, 4<<20)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	timeout := time.After(20 * time.Second)
	for len(got) < 3 {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("stdout closed early; stderr: %s", stderr.String())
			}
			var m msg
			if err := json.Unmarshal([]byte(line), &m); err != nil || m.JSONRPC != "2.0" {
				t.Fatalf("non JSON-RPC line on stdout: %q", line)
			}
			if m.Error != nil {
				t.Fatalf("id %d: %s", m.ID, m.Error)
			}
			got[m.ID] = m
		case <-timeout:
			t.Fatalf("no answer in 20 s; got %d; stderr: %s", len(got), stderr.String())
		}
	}
	if !strings.Contains(string(got[1].Result), `"codedb-lens"`) {
		t.Errorf("initialize: %s", got[1].Result)
	}
	for _, name := range []string{"list_profiles", "list_rules", "explain_rule"} {
		if !strings.Contains(string(got[2].Result), `"name":"`+name+`"`) {
			t.Errorf("tools/list lacks %s", name)
		}
	}
	if !strings.Contains(string(got[3].Result), "MSSQL-BACKUP-001") {
		t.Errorf("list_rules: %.300s", got[3].Result)
	}

	// The client closing stdin ends the session: clean exit, nothing more on stdout.
	stdin.Close()
	for line := range lines {
		t.Errorf("stdout after the last answer: %q", line)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("exit: %v; stderr: %s", err, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Error("did not exit 10 s after stdin closed")
	}
}
