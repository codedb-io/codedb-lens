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

	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/mcpserver"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/rules"
)

// mcpCmd runs the MCP server on stdin/stdout. The AI client starts this process and talks JSON-RPC
// over the pipes, so nothing else may be written to stdout: no update check, no banner, no progress.
func mcpCmd(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	profiles := fs.String("profile", "", "comma-separated profiles the AI may use (default: all saved profiles)")
	lang := fs.String("lang", "", "en or pt-BR (default: system language)")
	rulesDir := fs.String("rules-dir", "", "use rules from this folder instead of the built-in catalog")
	_ = fs.Parse(args)

	o := mcpserver.Options{Version: version, Lang: i18n.Normalize(firstNonEmpty(*lang, i18n.Detect())), Catalog: catalogFS(*rulesDir)}
	if *profiles != "" {
		saved, err := profile.Load()
		if err != nil {
			return err
		}
		for _, n := range strings.Split(*profiles, ",") {
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

func catalogFS(rulesDir string) fs.FS {
	if rulesDir != "" {
		return os.DirFS(rulesDir)
	}
	return rules.FS
}
