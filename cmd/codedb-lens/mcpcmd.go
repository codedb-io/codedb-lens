package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
	var c checkOpts
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	connFlags(fs, &c)
	fs.Lookup("profile").Usage = "comma-separated saved profiles the AI may use (default: all)"
	fs.Lookup("engine").Usage = "mssql or pg: use this connection instead of saved profiles (servers, Docker)"
	rulesDir := fs.String("rules-dir", "", "use rules from this folder instead of the built-in catalog")
	redact := fs.Bool("redact", false, "mask host, database, object, login and file names in check, compare and recording results")
	_ = fs.Parse(args)

	o := mcpOptions(i18n.Normalize(firstNonEmpty(c.lang, i18n.Detect())), *rulesDir, *redact)
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
