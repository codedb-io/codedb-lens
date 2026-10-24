package mcpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/profile"
)

// Write mode (Q44–Q46). execute_sql exists only with both keys: the profile's mcp_write (codedb-lens mcp
// allow-write) and --allow-write on the line the AI client runs. The user confirms every call (elicitation),
// dry_run is the default, a deny list holds even then, and every attempt goes to mcp-audit.jsonl.

func addWriteTool(m *mcp.Server, s *server) {
	yes := true
	mcp.AddTool(m, &mcp.Tool{Name: "execute_sql", Annotations: &mcp.ToolAnnotations{DestructiveHint: &yes},
		Description: "Change the database of a profile where the user turned writing on (list_profiles: write_enabled). The user sees the SQL and confirms every call. dry_run defaults to true: the statement runs in a transaction, you get the affected rows, and it is rolled back; call again with dry_run: false to apply it. Statements that cannot run in a transaction (VACUUM, CREATE INDEX CONCURRENTLY, ALTER DATABASE, BACKUP) have no preview. Always refused: COMMIT/ROLLBACK, logins, roles, permissions, impersonation, server settings, DROP DATABASE, file or OS access, code outside the database and dynamic SQL; so are such words in comments or strings."},
		s.execute)
}

// writeEnabled: both keys turned, on a saved profile (never the connection from flags).
func (s *server) writeEnabled(p profile.Profile) bool {
	return s.AllowWrite != "" && s.Fixed == nil && p.MCPWrite
}

type executeIn struct {
	dbIn
	SQL      string `json:"sql" jsonschema:"the statement to run"`
	DryRun   *bool  `json:"dry_run,omitempty" jsonschema:"default true: run in a transaction, report the affected rows and roll back; false applies it"`
	TimeoutS int    `json:"timeout_s,omitempty" jsonschema:"seconds; default and ceiling: the profile's"`
}

type executeOut struct {
	RowsAffected int64  `json:"rows_affected"`
	RolledBack   bool   `json:"rolled_back"`
	ElapsedMS    int64  `json:"elapsed_ms"`
	Note         string `json:"note,omitempty"`
}

// auditLine is one attempt in mcp-audit.jsonl, next to profiles.json.
type auditLine struct {
	Time       time.Time `json:"time"`
	Profile    string    `json:"profile"`
	Database   string    `json:"database,omitempty"`
	SQL        string    `json:"sql"`
	SHA256     string    `json:"sha256"`
	Mode       string    `json:"mode"`   // preview, commit, direct
	Result     string    `json:"result"` // ok, error, declined, refused
	Rows       int64     `json:"rows"`
	RolledBack bool      `json:"rolled_back"`
	Error      string    `json:"error,omitempty"`
}

func AuditPath() (string, error) {
	p, err := profile.Path()
	return filepath.Join(filepath.Dir(p), "mcp-audit.jsonl"), err
}

func openAudit() (*os.File, error) {
	path, err := AuditPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
}

func (s *server) execute(ctx context.Context, req *mcp.CallToolRequest, in executeIn) (*mcp.CallToolResult, executeOut, error) {
	var out executeOut
	p, err := s.profile(in.Profile)
	if err != nil {
		return nil, out, err
	}
	lang := s.lang(p)
	f, err := openAudit() // fail closed: no audit, no write
	if err != nil {
		return nil, out, fmt.Errorf(i18n.T(lang, "mcp_write_audit"), err)
	}
	defer f.Close()
	sum := sha256.Sum256([]byte(in.SQL))
	dry := in.DryRun == nil || *in.DryRun
	a := auditLine{Time: time.Now().UTC(), Profile: p.Name, Database: in.Database, SQL: in.SQL, SHA256: hex.EncodeToString(sum[:]), Mode: "commit"}
	log := func() error {
		raw, _ := json.Marshal(a)
		_, err := f.Write(append(raw, '\n'))
		return err
	}
	done := func(result string, err error) (*mcp.CallToolResult, executeOut, error) {
		a.Result = result
		if err != nil {
			a.Error = err.Error()
		}
		if werr := log(); werr != nil && err == nil {
			err = fmt.Errorf(i18n.T(lang, "mcp_write_audit"), werr)
		}
		return nil, out, err
	}

	if !s.writeEnabled(p) { // revoked during the session, or a profile without the key
		return done("refused", fmt.Errorf(i18n.T(lang, "mcp_write_off"), p.Name))
	}
	word, direct, ok := writeGuard(p.Conn.Engine, in.SQL)
	if !ok {
		return done("refused", fmt.Errorf(i18n.T(lang, "mcp_write_denied"), word))
	}
	mode := adapter.Commit
	switch {
	case direct && dry:
		a.Mode = "direct"
		return done("refused", errors.New(i18n.T(lang, "mcp_write_direct")))
	case direct:
		mode, a.Mode = adapter.Direct, "direct"
	case dry:
		mode, a.Mode = adapter.Preview, "preview"
	}
	ask, err := s.confirm(req, p, in, a.Mode, a.SHA256, lang)
	if ask != nil { // first round: the client shows the SQL and calls again with the answer
		return ask, out, nil
	}
	if err != nil {
		return done("declined", err)
	}
	secs, _ := limit(in.TimeoutS, p.MCPTimeoutS, defTimeoutS, capTimeoutS)
	p, sess, err := s.open(ctx, in.dbIn)
	if err != nil {
		return done("error", err)
	}
	defer sess.Close()
	a.Result = "started" // no log line, no write: the result line follows
	if err := log(); err != nil {
		return nil, out, fmt.Errorf(i18n.T(lang, "mcp_write_audit"), err)
	}
	start := time.Now()
	n, err := sess.Write(ctx, in.SQL, mode, time.Duration(secs)*time.Second)
	out.ElapsedMS = time.Since(start).Milliseconds()
	if err != nil {
		if adapter.Denied(err) {
			err = fmt.Errorf(i18n.T(lang, "mcp_denied"), p.Name, err)
		}
		a.Rows, a.RolledBack = n, mode != adapter.Direct && !errors.Is(err, adapter.ErrTxEnded)
		return done("error", err)
	}
	out.RowsAffected, a.Rows = n, n
	if mode == adapter.Preview {
		out.RolledBack, a.RolledBack, out.Note = true, true, i18n.T(lang, "mcp_write_preview_note")
	}
	return done("ok", nil)
}

// pendingWrites ties a confirmation to the question it answers: the RequestState the server handed out for
// this profile, database, mode and SQL, used once. A client cannot send a ready-made "yes" on the first call.
type pendingWrites struct {
	mu sync.Mutex
	m  map[string]pendingWrite
}

type pendingWrite struct {
	key string
	at  time.Time
}

func (pw *pendingWrites) add(key string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := hex.EncodeToString(b)
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if pw.m == nil {
		pw.m = map[string]pendingWrite{}
	}
	for k, v := range pw.m { // ponytail: unanswered questions are few; sweep them here
		if time.Since(v.at) > 10*time.Minute {
			delete(pw.m, k)
		}
	}
	pw.m[state] = pendingWrite{key, time.Now()}
	return state
}

func (pw *pendingWrites) take(state, key string) bool {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	v, ok := pw.m[state]
	delete(pw.m, state)
	return ok && v.key == key && time.Since(v.at) <= 10*time.Minute
}

// confirm asks the user through the client (multi round-trip, SEP-2322; the SDK turns it into elicitation/create
// for older clients). The first round returns the question; the second must carry the user's "yes" and the
// state of that question. A client without elicitation is refused, unless the user chose
// --allow-write=no-confirm (then the client's own per-tool approval is the only prompt).
func (s *server) confirm(req *mcp.CallToolRequest, p profile.Profile, in executeIn, mode, sum, lang string) (*mcp.CallToolResult, error) {
	db := in.Database
	if db == "" {
		db = p.Conn.Database
	}
	// The answer holds for this exact connection too: a profile edited to point elsewhere asks again.
	c := p.Conn
	key := strings.Join([]string{p.Name, c.Engine, c.Server, strconv.Itoa(c.Port), c.User, db, mode, sum}, "/")
	answer, answered := req.Params.InputResponses["confirm"]
	if !answered {
		ip := req.Session.InitializeParams()
		if ip == nil || ip.Capabilities == nil || ip.Capabilities.Elicitation == nil {
			if s.AllowWrite == "no-confirm" {
				return nil, nil
			}
			return nil, errors.New(i18n.T(lang, "mcp_write_no_elicit"))
		}
		return &mcp.CallToolResult{RequestState: s.pending.add(key), InputRequests: mcp.InputRequestMap{"confirm": &mcp.ElicitParams{
			Message: fmt.Sprintf(i18n.T(lang, "mcp_write_ask"), p.Name, c.Target(), db, i18n.T(lang, "mcp_write_mode_"+mode), in.SQL),
			RequestedSchema: map[string]any{"type": "object", "required": []string{"confirm"}, "properties": map[string]any{
				"confirm": map[string]any{"type": "boolean", "title": i18n.T(lang, "mcp_write_confirm"), "default": false}}},
		}}}, nil
	}
	r, _ := answer.(*mcp.ElicitResult)
	if !s.pending.take(req.Params.RequestState, key) || r == nil || r.Action != "accept" || r.Content["confirm"] != true {
		return nil, errors.New(i18n.T(lang, "mcp_write_declined"))
	}
	return nil, nil
}
