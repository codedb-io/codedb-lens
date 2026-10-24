// Package mcpserver exposes the Lens to AI clients over the Model Context Protocol (stdio).
// Read-only: no tool here writes to a database or to any configuration.
package mcpserver

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/profile"
)

type Options struct {
	Version  string
	Lang     string   // language of rule texts and error messages
	Profiles []string // profiles the AI may see; empty = all saved profiles
	Catalog  fs.FS    // rules.FS or --rules-dir
}

const instructions = `codeDB Lens: read-only health checks for SQL Server and PostgreSQL.
Nothing you call here changes a database or any configuration.
Connections come from the user's saved profiles (list_profiles); passwords are never shown.
Use list_rules and explain_rule to explain what a check measures and how to fix a finding.`

type server struct {
	Options
	rules []*engine.Rule
}

// New builds the MCP server. The catalog is loaded once; profiles are read on every call,
// so a profile saved in the GUI during the session shows up without a restart.
func New(o Options) (*mcp.Server, error) {
	s := &server{Options: o}
	for _, eng := range []string{"mssql", "pg"} {
		for _, load := range []func(fs.FS, string) ([]*engine.Rule, error){engine.LoadCatalog, engine.LoadSeries} {
			rs, err := load(o.Catalog, eng)
			if err != nil {
				return nil, err
			}
			s.rules = append(s.rules, rs...)
		}
	}
	m := mcp.NewServer(&mcp.Implementation{Name: "codedb-lens", Title: "codeDB Lens", Version: o.Version,
		WebsiteURL: "https://lens.codedb.com.br/"}, &mcp.ServerOptions{Instructions: instructions})
	ro := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(m, &mcp.Tool{Name: "list_profiles", Annotations: ro,
		Description: "List the saved connection profiles you may use: name, engine, host, initial database and language. Never returns passwords or connection strings."},
		s.listProfiles)
	mcp.AddTool(m, &mcp.Tool{Name: "list_rules", Annotations: ro,
		Description: "List the Lens rule catalog (health checks): ID, engine, category, severity and title. Kind \"series\" rules judge recordings, the others run in a check."},
		s.listRules)
	mcp.AddTool(m, &mcp.Tool{Name: "explain_rule", Annotations: ro,
		Description: "Explain one rule: what it measures, why it matters, the next action, how to verify the fix, the permission it needs, its SQL and its documentation link."},
		s.explainRule)
	return m, nil
}

type profileOut struct {
	Name     string `json:"name"`
	Engine   string `json:"engine"`
	Host     string `json:"host"`
	Database string `json:"database,omitempty"`
	Lang     string `json:"lang,omitempty"`
}

type listProfilesOut struct {
	Profiles []profileOut `json:"profiles"`
}

func (s *server) listProfiles(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, listProfilesOut, error) {
	out := listProfilesOut{Profiles: []profileOut{}}
	f, err := profile.Load()
	if err != nil {
		return nil, out, err
	}
	for _, p := range f.Profiles {
		if s.visible(p.Name) {
			out.Profiles = append(out.Profiles, profileOut{p.Name, p.Conn.Engine, p.Conn.Server, p.Conn.Database, p.Lang})
		}
	}
	return nil, out, nil
}

// visible applies --profile. Later tools that take a profile argument must refuse hidden ones too.
func (s *server) visible(name string) bool {
	if len(s.Profiles) == 0 {
		return true
	}
	for _, n := range s.Profiles {
		if strings.EqualFold(n, name) { // same rule as profile.Get
			return true
		}
	}
	return false
}

type listRulesIn struct {
	Engine string `json:"engine,omitempty" jsonschema:"mssql or pg; empty lists both"`
}

type ruleSummary struct {
	ID       string          `json:"id"`
	Engine   string          `json:"engine"`
	Kind     string          `json:"kind,omitempty"`
	Category string          `json:"category"`
	Severity engine.Severity `json:"severity"`
	Title    string          `json:"title"`
}

type listRulesOut struct {
	Rules []ruleSummary `json:"rules"`
}

func (s *server) listRules(_ context.Context, _ *mcp.CallToolRequest, in listRulesIn) (*mcp.CallToolResult, listRulesOut, error) {
	out := listRulesOut{Rules: []ruleSummary{}}
	if in.Engine != "" && in.Engine != "mssql" && in.Engine != "pg" {
		return nil, out, fmt.Errorf(i18n.T(s.Lang, "mcp_engine_invalid"), in.Engine)
	}
	for _, r := range s.rules {
		if in.Engine == "" || r.Engine == in.Engine {
			out.Rules = append(out.Rules, ruleSummary{r.ID, r.Engine, r.Kind, r.Category, r.Severity, r.Title[s.Lang]})
		}
	}
	return nil, out, nil
}

type explainRuleIn struct {
	RuleID string `json:"rule_id" jsonschema:"rule ID, e.g. MSSQL-BACKUP-001 or PG-AUTOVAC-001"`
}

type ruleOut struct {
	ruleSummary
	Impact     string `json:"impact"`
	NextAction string `json:"next_action"`
	Verify     string `json:"verify"`
	Caveat     string `json:"caveat,omitempty"`
	Permission string `json:"permission"`
	SQL        string `json:"sql,omitempty"` // empty for series rules, which read a recording
	DocURL     string `json:"doc_url"`
}

func (s *server) explainRule(_ context.Context, _ *mcp.CallToolRequest, in explainRuleIn) (*mcp.CallToolResult, ruleOut, error) {
	for _, r := range s.rules {
		if strings.EqualFold(r.ID, in.RuleID) {
			l := s.Lang
			return nil, ruleOut{ruleSummary{r.ID, r.Engine, r.Kind, r.Category, r.Severity, r.Title[l]},
				r.Impact[l], r.NextAction[l], r.Verify[l], r.Caveat[l], r.Permission, r.SQL, r.DocURL}, nil
		}
	}
	return nil, ruleOut{}, fmt.Errorf(i18n.T(s.Lang, "mcp_rule_unknown"), in.RuleID)
}
