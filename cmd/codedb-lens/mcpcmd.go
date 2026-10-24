package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/term"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/mcpserver"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/internal/report"
	"github.com/codedb-io/codedb-lens/rules"
)

// mcpCmd runs the MCP server on stdin/stdout. The AI client starts this process and talks JSON-RPC
// over the pipes, so nothing else may be written to stdout: no update check, no banner, no progress.
func mcpCmd(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "install", "uninstall":
			return mcpInstallCmd(args[0], args[1:])
		case "status":
			return mcpStatusCmd()
		case "allow-write", "revoke-write":
			return mcpWriteCmd(args[0], args[1:])
		}
	}
	var c checkOpts
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	connFlags(fs, &c)
	fs.Lookup("profile").Usage = "comma-separated saved profiles the AI may use (default: all)"
	fs.Lookup("engine").Usage = "mssql or pg: use this connection instead of saved profiles (servers, Docker)"
	rulesDir := fs.String("rules-dir", "", "use rules from this folder instead of the built-in catalog")
	redact := fs.Bool("redact", false, "mask host, database, object, login and file names in check, compare and recording results")
	var write writeFlag
	fs.Var(&write, "allow-write", "with a profile allowed by mcp allow-write: list execute_sql (=no-confirm also for clients without confirmation prompts)")
	_ = fs.Parse(args)

	o := mcpOptions(i18n.Normalize(firstNonEmpty(c.lang, i18n.Detect())), *rulesDir, *redact)
	o.AllowWrite = string(write)
	switch {
	case c.conn.Engine != "": // no profiles: the connection comes from flags and env
		if c.dsnEnv != "" {
			if c.conn.DSN = os.Getenv(c.dsnEnv); c.conn.DSN == "" {
				return fmt.Errorf("environment variable %s is empty", c.dsnEnv)
			}
		}
		if err := readPassword(&c); err != nil {
			return err
		}
		o.Fixed = &profile.Profile{Name: "default", Conn: c.conn, Lang: c.lang}
	case c.profile != "":
		saved, err := profile.Load()
		if err != nil {
			return err
		}
		for _, n := range strings.Split(c.profile, ",") {
			n = strings.TrimSpace(n)
			if _, ok := saved.Get(n); !ok {
				return fmt.Errorf(i18n.T(o.Lang, "mcp_profile_unknown"), n)
			}
			o.Profiles = append(o.Profiles, n)
		}
	}
	srv, err := mcpserver.New(o)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return srv.Run(ctx, &mcp.StdioTransport{})
}

// stdinTerminal is replaced by the tests, which type through a pipe.
var stdinTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// writeFlag is --allow-write (confirm) or --allow-write=no-confirm.
type writeFlag string

func (w *writeFlag) String() string   { return string(*w) }
func (w *writeFlag) IsBoolFlag() bool { return true }
func (w *writeFlag) Set(v string) error {
	switch v {
	case "true":
		*w = "confirm"
	case "no-confirm":
		*w = "no-confirm"
	case "false":
		*w = ""
	default:
		return fmt.Errorf("--allow-write takes no value or =no-confirm, not %q", v)
	}
	return nil
}

// mcpWriteCmd turns the profile's key of the write mode on (after the user types the profile name) or off.
func mcpWriteCmd(verb string, args []string) error {
	lang := i18n.Normalize(i18n.Detect())
	fs := flag.NewFlagSet("mcp "+verb, flag.ExitOnError)
	name := fs.String("profile", "", "saved profile")
	_ = fs.Parse(args)
	f, err := profile.Load()
	if err != nil {
		return err
	}
	p, ok := f.Get(*name)
	if !ok {
		return fmt.Errorf(i18n.T(lang, "mcp_profile_unknown"), *name)
	}
	if verb == "allow-write" {
		if !stdinTerminal() { // a person types it; an AI client with a shell tool has no terminal to type in
			return errors.New(i18n.T(lang, "mcp_allow_write_tty"))
		}
		fmt.Printf(i18n.T(lang, "mcp_allow_write_ask"), p.Name)
		typed, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(typed) != p.Name {
			return errors.New(i18n.T(lang, "mcp_allow_write_mismatch"))
		}
	}
	last := f.Last
	p.MCPWrite = verb == "allow-write"
	f.Put(p)
	f.Last = last // Put marks the profile as last used; this is not a use
	if err := f.Save(); err != nil {
		return err
	}
	key := "mcp_write_revoked"
	if p.MCPWrite {
		key = "mcp_write_allowed"
	}
	fmt.Printf(i18n.T(lang, key)+"\n", p.Name)
	return nil
}

// mcpInstallCmd adds or removes the Lens in an AI client's settings. Claude Code gets its own command instead.
func mcpInstallCmd(verb string, args []string) error {
	lang := i18n.Normalize(i18n.Detect())
	fs := flag.NewFlagSet("mcp "+verb, flag.ExitOnError)
	client := fs.String("client", "", "claude-desktop, claude-code, vscode or cursor")
	profiles := fs.String("profile", "", "comma-separated saved profiles the AI may use (default: all)")
	redact := fs.Bool("redact", false, "mask names in check, compare and recording results")
	dry := fs.Bool("dry-run", false, "show the change without writing it")
	_ = fs.Parse(args)
	cl, ok := mcpserver.FindClient(*client)
	if !ok {
		return fmt.Errorf(i18n.T(lang, "mcp_client_unknown"), *client)
	}
	var names []string
	if *profiles != "" {
		saved, err := profile.Load()
		if err != nil {
			return err
		}
		for _, n := range strings.Split(*profiles, ",") {
			p, ok := saved.Get(strings.TrimSpace(n))
			if !ok {
				return fmt.Errorf(i18n.T(lang, "mcp_profile_unknown"), n)
			}
			names = append(names, p.Name)
		}
	}
	exe, err := os.Executable() // absolute: the installer leaves the Lens off PATH by default
	if err != nil {
		return err
	}
	mcpArgs := mcpserver.Args(names, *redact)
	if cl.ID == "claude-code" {
		cmd := mcpserver.ClaudeCodeCommand(exe, mcpArgs)
		if verb == "uninstall" {
			cmd = "claude mcp remove --scope user " + mcpserver.ServerName
		}
		fmt.Printf("%s\n\n  %s\n", i18n.T(lang, "mcp_claude_code"), cmd)
		return nil
	}
	var ch mcpserver.Change
	if verb == "install" {
		ch, err = cl.Install(exe, mcpArgs, *dry)
	} else {
		ch, err = cl.Uninstall(*dry)
	}
	if errors.Is(err, mcpserver.ErrInvalidJSON) {
		return fmt.Errorf(i18n.T(lang, "mcp_install_invalid"), ch.File, err, cl.Snippet(exe, mcpArgs))
	}
	if err != nil {
		return err
	}
	switch {
	case *dry:
		fmt.Printf(i18n.T(lang, "mcp_install_dry")+"\n- %s\n+ %s\n", cl.Name, ch.File, firstNonEmpty(ch.Before, "(none)"), firstNonEmpty(ch.After, "(none)"))
	case !ch.Changed && verb == "install":
		fmt.Printf(i18n.T(lang, "mcp_install_same")+"\n", cl.Name, ch.File)
	case !ch.Changed:
		fmt.Printf(i18n.T(lang, "mcp_uninstall_none")+"\n", cl.Name, ch.File)
	default:
		key := "mcp_install_done"
		if verb == "uninstall" {
			key = "mcp_uninstall_done"
		}
		fmt.Printf(i18n.T(lang, key)+"\n", cl.Name, ch.File, cl.Name)
		if ch.Backup != "" {
			fmt.Printf(i18n.T(lang, "mcp_install_backup")+"\n", ch.Backup)
		}
	}
	return nil
}

// mcpGuide is one client's block in the GUI's "AI (MCP)" screen: the command to copy and, for clients whose file
// the Lens edits, the JSON to paste by hand. The GUI only shows them; installing stays on the command line (Q48).
type mcpGuide struct {
	Client  string `json:"client"`
	Name    string `json:"name"`
	Command string `json:"command"`
	Snippet string `json:"snippet,omitempty"`
}

// mcpGuides builds the same entries as mcp install, from the same generator.
func mcpGuides(exe string, profiles []string, redact bool) []mcpGuide {
	args := mcpserver.Args(profiles, redact)
	var out []mcpGuide
	for _, c := range mcpserver.Clients {
		g := mcpGuide{Client: c.ID, Name: c.Name}
		if c.ID == "claude-code" {
			g.Command = mcpserver.ClaudeCodeCommand(exe, args)
		} else {
			g.Command = mcpserver.Quote(exe) + " mcp install --client " + c.ID
			if len(profiles) > 0 {
				g.Command += " --profile " + mcpserver.Quote(strings.Join(profiles, ","))
			}
			if redact {
				g.Command += " --redact"
			}
			g.Snippet = c.Snippet(exe, args)
		}
		out = append(out, g)
	}
	return out
}

func mcpStatusCmd() error {
	lang := i18n.Normalize(i18n.Detect())
	for _, st := range mcpserver.Status() {
		var line string
		switch {
		case st.Error != "":
			line = fmt.Sprintf(i18n.T(lang, "mcp_status_error"), st.File, st.Error)
		case st.Missing:
			line = fmt.Sprintf(i18n.T(lang, "mcp_status_missing"), st.Command)
		case st.Configured:
			line = fmt.Sprintf(i18n.T(lang, "mcp_status_ok"), st.Command)
		case st.Found:
			line = fmt.Sprintf(i18n.T(lang, "mcp_status_absent"), st.File)
		default:
			line = fmt.Sprintf(i18n.T(lang, "mcp_status_nofile"), st.File)
		}
		fmt.Printf("%-15s %s\n", st.Name, line)
	}
	f, err := profile.Load()
	for _, p := range f.Profiles {
		if p.MCPWrite {
			fmt.Printf(i18n.T(lang, "mcp_status_write")+"\n", p.Name)
		}
	}
	return err
}

// mcpOptions wires the server to the same diagnosis, history and compare as the CLI and the GUI.
func mcpOptions(lang, rulesDir string, redact bool) mcpserver.Options {
	return mcpserver.Options{
		Version: version, Lang: lang, Catalog: catalogFS(rulesDir), Redact: redact,
		Check: func(ctx context.Context, p profile.Profile, c adapter.Conn, lang string, redact bool) (*report.Report, string, error) {
			rep, err := Diagnose(ctx, c, lang, rulesDir, func(string) {}, nil)
			if err != nil {
				return nil, "", err
			}
			path, err := saveReport(rep, firstNonEmpty(p.OutDir, report.DefaultDir(p.Name)), redact)
			return rep, path, err
		},
		Compare: func(lang, a, b string) (*report.Comparison, string, error) { return compareFiles(lang, a, b, "") },
	}
}

func catalogFS(rulesDir string) fs.FS {
	if rulesDir != "" {
		return os.DirFS(rulesDir)
	}
	return rules.FS
}
