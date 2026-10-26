// Package mcpserver exposes the Lens to AI clients over the Model Context Protocol (stdio).
// Read-only by default: only execute_sql writes, and it exists only with the double opt-in of write.go.
// No tool changes any configuration.
package mcpserver

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/internal/report"
)

type Options struct {
	Version  string
	Lang     string   // language of rule texts and error messages
	Profiles []string // profiles the AI may see; empty = all saved profiles
	Catalog  fs.FS    // rules.FS or --rules-dir
	Redact   bool     // mask names in check, compare and recording results (--redact)
	// Fixed replaces the saved profiles with one connection from flags (Docker, servers); its password is in Conn.
	Fixed *profile.Profile
	// AllowWrite is --allow-write: "", "confirm" or "no-confirm". The other key is the profile's mcp_write.
	AllowWrite string

	// Check diagnoses and saves the report in the profile's history, like the GUI (main's Diagnose + saveReport).
	Check func(ctx context.Context, p profile.Profile, c adapter.Conn, lang string, redact bool) (*report.Report, string, error)
	// Compare is main's compareFiles: free key, instance limit, diff, save.
	Compare func(lang, fileA, fileB string) (*report.Comparison, string, error)
}

const instructions = `codeDB Lens: read-only health checks for SQL Server and PostgreSQL.
Nothing you call here changes a database or any configuration.
Connections come from the user's saved profiles (list_profiles); passwords are never shown.
check runs the diagnosis (10-60 s) and saves the report in the profile's history; get_finding gives the full evidence.
A finding is Confirmed or a Hint; "no data" means the rule could not run (permissions, platform), never "OK".
Recordings are started by the user in a terminal (record_command); you read them with recording_report.
For data questions: list_tables, describe_table, then query with one SELECT that aggregates in SQL.
query is read-only and stops at the profile's row limit; truncated: true means a partial answer, so refine and ask again.`

type server struct {
	Options
	rules   []*engine.Rule
	pending pendingWrites // execute_sql questions waiting for the user's answer
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
		WebsiteURL: "https://lens.codedb.com.br/"}, &mcp.ServerOptions{Instructions: s.instructions()})
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
	addDiagTools(m, s, ro)
	addRecordingTools(m, s, ro)
	addDataTools(m, s, ro)
	if s.anyWrite() {
		addWriteTool(m, s)
	}
	addPrompts(m)
	return m, nil
}

// anyWrite: execute_sql is listed only when both keys are turned for some profile the AI may use (Q44).
func (s *server) anyWrite() bool {
	ps, _ := s.all()
	for _, p := range ps {
		if s.writeEnabled(p) {
			return true
		}
	}
	return false
}

func (s *server) instructions() string {
	if !s.anyWrite() {
		return instructions
	}
	return strings.Replace(instructions, "Nothing you call here changes a database or any configuration.",
		"Only execute_sql changes a database, on profiles with write_enabled: the user confirms each call and dry_run (the default) rolls back. Nothing changes any configuration.", 1)
}

type profileOut struct {
	Name     string `json:"name"`
	Engine   string `json:"engine"`
	Host     string `json:"host"`
	Database string `json:"database,omitempty"`
	Lang     string `json:"lang,omitempty"`
	Write    bool   `json:"write_enabled"` // execute_sql may change this profile's database
}

type listProfilesOut struct {
	Profiles []profileOut `json:"profiles"`
	Note     string       `json:"note,omitempty"`
}

func (s *server) listProfiles(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, listProfilesOut, error) {
	out := listProfilesOut{Profiles: []profileOut{}}
	ps, err := s.all()
	for _, p := range ps {
		out.Profiles = append(out.Profiles, profileOut{p.Name, p.Conn.Engine, p.Conn.Server, p.Conn.Database, p.Lang, s.writeEnabled(p)})
	}
	if s.AllowWrite != "" && !s.anyWrite() { // only one of the two keys: say so instead of leaving the AI to guess
		out.Note = "The client started the Lens with --allow-write, but no profile here allows writing, so execute_sql is not available. " +
			"The user turns it on for a saved profile by typing codedb-lens mcp allow-write --profile NAME in a terminal, then restarts the client."
	}
	return nil, out, err
}

// all returns the profiles the AI may use: the fixed connection, or the saved profiles filtered by --profile.
func (s *server) all() ([]profile.Profile, error) {
	if s.Fixed != nil {
		return []profile.Profile{*s.Fixed}, nil
	}
	f, err := profile.Load()
	var out []profile.Profile
	for _, p := range f.Profiles {
		if s.visible(p.Name) {
			out = append(out, p)
		}
	}
	return out, err
}

// visible applies --profile.
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

// profile finds a profile the AI may use; hidden and missing ones get the same answer.
func (s *server) profile(name string) (profile.Profile, error) {
	ps, err := s.all()
	if err != nil {
		return profile.Profile{}, err
	}
	for _, p := range ps {
		if strings.EqualFold(p.Name, name) {
			return p, nil
		}
	}
	return profile.Profile{}, fmt.Errorf(i18n.T(s.Lang, "mcp_profile_unavailable"), name)
}

// conn adds the password: OS credential store, then CODEDB_LENS_PASSWORD. Never a prompt: stdin is the protocol.
func (s *server) conn(p profile.Profile) (adapter.Conn, error) {
	c := p.Conn
	if c.Password != "" || c.DSN != "" || c.Auth == "windows" && c.Engine == "mssql" {
		return c, nil
	}
	if pw, err := profile.Password(p.Name); err == nil {
		c.Password = pw
	} else if c.Password = os.Getenv("CODEDB_LENS_PASSWORD"); c.Password == "" {
		return c, fmt.Errorf(i18n.T(s.Lang, "mcp_no_password"), p.Name)
	}
	return c, nil
}

// lang is the profile's language, else the server's.
func (s *server) lang(p profile.Profile) string {
	if p.Lang != "" {
		return i18n.Normalize(p.Lang)
	}
	return s.Lang
}

// reportsDir is where check, the GUI and compare keep a profile's runs.
func reportsDir(p profile.Profile) string {
	if p.OutDir != "" {
		return p.OutDir
	}
	return report.DefaultDir(p.Name)
}

func recordingsDir(p profile.Profile) string { return filepath.Join(reportsDir(p), "recordings") }

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
