package mcpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ServerName is the Lens entry in every AI client's MCP settings.
const ServerName = "codedb-lens"

// Client is an AI client whose MCP settings mcp install edits, and mcp status and the GUI read.
// Claude Code manages its own settings: install prints its command, and its file is only read.
type Client struct {
	ID, Name string
	path     func() (string, error) // settings file
	key      string                 // object that holds the servers
	typed    bool                   // entries carry "type": "stdio" (VS Code)
}

var Clients = []Client{
	{"claude-desktop", "Claude Desktop", configFile("Claude", "claude_desktop_config.json"), "mcpServers", false},
	{"claude-code", "Claude Code", homeFile(".claude.json"), "mcpServers", false},
	{"vscode", "VS Code", configFile("Code", "User", "mcp.json"), "servers", true},
	{"cursor", "Cursor", homeFile(".cursor", "mcp.json"), "mcpServers", false},
}

// configFile is under %APPDATA%, ~/Library/Application Support or ~/.config.
// ponytail: the Microsoft Store build of Claude Desktop keeps a virtualized copy; add its path if users hit it.
func configFile(parts ...string) func() (string, error) {
	return func() (string, error) {
		dir, err := os.UserConfigDir()
		return filepath.Join(append([]string{dir}, parts...)...), err
	}
}

func homeFile(parts ...string) func() (string, error) {
	return func() (string, error) {
		dir, err := os.UserHomeDir()
		return filepath.Join(append([]string{dir}, parts...)...), err
	}
}

func FindClient(id string) (Client, bool) {
	for _, c := range Clients {
		if c.ID == id {
			return c, true
		}
	}
	return Client{}, false
}

// Args are the arguments the client starts the Lens with.
func Args(profiles []string, redact bool) []string {
	a := []string{"mcp"}
	if len(profiles) > 0 {
		a = append(a, "--profile", strings.Join(profiles, ","))
	}
	if redact {
		a = append(a, "--redact")
	}
	return a
}

func (c Client) entry(exe string, args []string) map[string]any {
	e := map[string]any{"command": exe, "args": args}
	if c.typed {
		e["type"] = "stdio"
	}
	return e
}

// ClaudeCodeCommand is what the user runs to add the Lens to Claude Code, for every project (user scope).
func ClaudeCodeCommand(exe string, args []string) string {
	parts := []string{"claude", "mcp", "add", "--scope", "user", ServerName, "--", Quote(exe)}
	for _, a := range args {
		parts = append(parts, Quote(a))
	}
	return strings.Join(parts, " ")
}

// Snippet is the JSON to paste by hand into the client's settings.
func (c Client) Snippet(exe string, args []string) string {
	raw, _ := json.MarshalIndent(map[string]any{c.key: map[string]any{ServerName: c.entry(exe, args)}}, "", "  ")
	return string(raw)
}

// ErrInvalidJSON: the settings file is not JSON (VS Code allows comments); it is left untouched.
var ErrInvalidJSON = errors.New("not valid JSON")

// Change is what Install or Uninstall did, or would do on a dry run. Before/After are our entry, "" when absent.
type Change struct {
	File, Backup  string
	Before, After string
	Changed       bool
}

// ErrManaged: Claude Code's settings are changed by its own command (ClaudeCodeCommand), never by the Lens.
var ErrManaged = errors.New("this client manages its own settings")

func (c Client) Install(exe string, args []string, dry bool) (Change, error) {
	if c.ID == "claude-code" {
		return Change{}, ErrManaged
	}
	return c.edit(c.entry(exe, args), dry)
}

func (c Client) Uninstall(dry bool) (Change, error) {
	if c.ID == "claude-code" {
		return Change{}, ErrManaged
	}
	return c.edit(nil, dry)
}

// edit sets (entry != nil) or removes our entry and keeps every other key; the old file goes to .bak.
// ponytail: top-level keys come back sorted (encoding/json); the values themselves are kept as they were.
func (c Client) edit(entry map[string]any, dry bool) (Change, error) {
	file, err := c.path()
	ch := Change{File: file}
	if err != nil {
		return ch, err
	}
	doc, servers, raw, err := c.read(file)
	if err != nil {
		return ch, err
	}
	if old, ok := servers[ServerName]; ok {
		ch.Before = canonical(old)
	}
	if entry == nil {
		delete(servers, ServerName)
	} else {
		b, _ := json.Marshal(entry)
		servers[ServerName], ch.After = b, string(b)
	}
	if ch.Changed = ch.Before != ch.After; !ch.Changed || dry {
		return ch, nil
	}
	doc[c.key], _ = json.Marshal(servers)
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return ch, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return ch, err
	}
	if raw != nil {
		ch.Backup = file + ".bak"
		if err := os.WriteFile(ch.Backup, raw, 0o600); err != nil {
			return ch, err
		}
	}
	return ch, os.WriteFile(file, append(out, '\n'), 0o600)
}

// read returns the settings document, its servers object and the raw file (nil when missing).
func (c Client) read(file string) (doc, servers map[string]json.RawMessage, raw []byte, err error) {
	doc, servers = map[string]json.RawMessage{}, map[string]json.RawMessage{}
	raw, err = os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return doc, servers, nil, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return doc, servers, raw, nil
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	if s, ok := doc[c.key]; ok {
		if err := json.Unmarshal(s, &servers); err != nil {
			return nil, nil, nil, fmt.Errorf("%w: %q: %v", ErrInvalidJSON, c.key, err)
		}
	}
	return doc, servers, raw, nil
}

// canonical re-encodes JSON with sorted keys, so a reordered but equal entry is not a change.
func canonical(raw json.RawMessage) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return string(raw)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// ClientStatus is one client's line in mcp status and the GUI.
type ClientStatus struct {
	Client     string `json:"client"`
	Name       string `json:"name"`
	File       string `json:"file"`
	Found      bool   `json:"found"`      // the settings file exists
	Configured bool   `json:"configured"` // it has our entry
	Command    string `json:"command,omitempty"`
	Missing    bool   `json:"missing"` // the recorded command no longer exists (moved by an update?)
	Error      string `json:"error,omitempty"`
}

func Status() []ClientStatus {
	var out []ClientStatus
	for _, c := range Clients {
		st := ClientStatus{Client: c.ID, Name: c.Name}
		file, err := c.path()
		st.File = file
		if err == nil {
			_, err = os.Stat(file)
			st.Found = err == nil
		}
		_, servers, _, rerr := c.read(file)
		switch {
		case err != nil && !errors.Is(err, os.ErrNotExist):
			st.Error = err.Error()
		case rerr != nil:
			st.Error = rerr.Error()
		default:
			if e, ok := servers[ServerName]; ok {
				var v struct{ Command string }
				_ = json.Unmarshal(e, &v)
				st.Configured, st.Command = true, v.Command
				_, serr := os.Stat(v.Command)
				st.Missing = v.Command != "" && serr != nil
			}
		}
		out = append(out, st)
	}
	return out
}
