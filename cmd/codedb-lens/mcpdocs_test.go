package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/codedb-io/codedb-lens/internal/mcpserver"
)

// The MCP pages and READMEs only cite commands, flags and clients the CLI has (plan §7.3.2): docs that drift
// from the code fail here.
func TestMCPDocsCommandsExist(t *testing.T) {
	src, err := os.ReadFile("mcpcmd.go")
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := os.ReadFile("main.go") // connFlags (--engine, --server ...) lives there
	code := string(src) + string(conn)
	files, _ := filepath.Glob("../../docs/*/mcp*.md")
	files = append(files, "../../README.md", "../../README.pt-BR.md")
	cmd := regexp.MustCompile(`codedb-lens(?:\.exe)?"? mcp\b([^\x60\n|]*)`)
	sub := regexp.MustCompile(`^\s+([a-z][a-z-]+)`)
	flag := regexp.MustCompile(`--([a-z][a-z-]*)`)
	client := regexp.MustCompile(`--client ([a-z-]+)`)
	n := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range cmd.FindAllStringSubmatch(string(raw), -1) {
			n++
			rest := m[1]
			if s := sub.FindStringSubmatch(rest); s != nil && !strings.Contains(code, `"`+s[1]+`"`) {
				t.Errorf("%s: unknown subcommand %q in %q", f, s[1], m[0])
			}
			for _, fl := range flag.FindAllStringSubmatch(rest, -1) {
				if !strings.Contains(code, `"`+fl[1]+`"`) {
					t.Errorf("%s: unknown flag --%s in %q", f, fl[1], m[0])
				}
			}
			for _, c := range client.FindAllStringSubmatch(rest, -1) {
				if _, ok := mcpserver.FindClient(c[1]); !ok {
					t.Errorf("%s: unknown client %q", f, c[1])
				}
			}
		}
	}
	if n < 10 {
		t.Fatalf("only %d commands found: the pattern no longer matches the docs", n)
	}
}
