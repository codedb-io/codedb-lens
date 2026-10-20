// Package engine loads the rule catalog, runs rules through an adapter and scores the result.
// It knows nothing about individual rules: verdicts come from the rule's SQL (rows = findings).
package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Text is a bilingual string. Both keys are mandatory (see Validate).
type Text map[string]string

var Languages = []string{"en", "pt-BR"}

type Severity string

const (
	Critical Severity = "critical"
	High     Severity = "high"
	Medium   Severity = "medium"
	Low      Severity = "low"
)

type Confidence string

const (
	Confirmed Confidence = "confirmed"
	Hint      Confidence = "hint"
	NoData    Confidence = "no_data"
)

type Rule struct {
	ID         string             `yaml:"id" json:"id"`
	Engine     string             `yaml:"engine" json:"engine"`
	Category   string             `yaml:"category" json:"category"`
	Severity   Severity           `yaml:"severity" json:"severity"`
	Confidence Confidence         `yaml:"confidence" json:"-"` // default for rows without a _confidence column
	MinMajor   int                `yaml:"min_major" json:"-"`
	MaxMajor   int                `yaml:"max_major" json:"-"`
	SkipOn     []string           `yaml:"skip_on" json:"-"`             // platforms where the rule doesn't apply
	Scope      string             `yaml:"scope" json:"scope,omitempty"` // "" = instance, "database" = run in each database
	Permission string             `yaml:"permission" json:"permission"`
	Timeout    time.Duration      `yaml:"timeout" json:"-"`
	Params     map[string]float64 `yaml:"params" json:"params,omitempty"`
	Title      Text               `yaml:"title" json:"title"`
	Impact     Text               `yaml:"impact" json:"impact"`
	NextAction Text               `yaml:"next_action" json:"next_action"`
	Caveat     Text               `yaml:"caveat" json:"caveat,omitempty"`
	SaaSCTA    Text               `yaml:"saas_cta" json:"saas_cta,omitempty"`
	DocURL     string             `json:"doc_url"`
	SQL        string             `yaml:"-" json:"query"`
}

// ServerInfo is what an adapter learns about the target before rules run.
type ServerInfo struct {
	Engine    string `json:"engine"`
	Version   string `json:"version"`
	Major     int    `json:"major"`
	Platform  string `json:"platform"` // onprem, azure_sqldb, azure_mi, rds, aurora, azure_flex ...
	Host      string `json:"host"`
	StartedAt string `json:"started_at,omitempty"` // stats reset on restart: a caveat for usage-based rules
}

// Adapter runs one read-only query and returns rows as ordered column/value pairs.
type Adapter interface {
	Info() ServerInfo
	Query(ctx context.Context, sql string) (cols []string, rows [][]any, err error)
	// Databases lists user databases the login can open, and those it can't (reported, never hidden).
	Databases(ctx context.Context) (open, noAccess []string, err error)
	QueryDB(ctx context.Context, db, sql string) (cols []string, rows [][]any, err error)
}

type Status string

const (
	StatusOK         Status = "ok"
	StatusFinding    Status = "finding"
	StatusNoData     Status = "no_data"
	StatusNotApplies Status = "not_applicable"
)

type Result struct {
	Rule        *Rule      `json:"rule"`
	Status      Status     `json:"status"`
	Confidence  Confidence `json:"confidence,omitempty"`
	Columns     []string   `json:"columns,omitempty"`
	Rows        [][]any    `json:"rows,omitempty"`
	Reason      string     `json:"reason,omitempty"` // why no data / not applicable, or which databases were skipped
	Duration    string     `json:"duration"`
	CollectedAt time.Time  `json:"collected_at"`
}

var placeholder = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

// LoadCatalog reads rules/<engine>/<ID>/{rule.yaml,query.sql} from fsys.
func LoadCatalog(fsys fs.FS, engine string) ([]*Rule, error) {
	dirs, err := fs.ReadDir(fsys, engine)
	if err != nil {
		return nil, err
	}
	var rules []*Rule
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		base := path.Join(engine, d.Name())
		raw, err := fs.ReadFile(fsys, path.Join(base, "rule.yaml"))
		if err != nil {
			return nil, err
		}
		r := &Rule{Timeout: 10 * time.Second, Confidence: Confirmed}
		if err := yaml.Unmarshal(raw, r); err != nil {
			return nil, fmt.Errorf("%s: %w", base, err)
		}
		q, err := fs.ReadFile(fsys, path.Join(base, "query.sql"))
		if err != nil {
			return nil, err
		}
		r.SQL = string(q)
		r.DocURL = "https://lens.codedb.com.br/rules/" + strings.ToLower(r.ID) + "/"
		if err := r.Validate(engine, d.Name()); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules, nil
}

// Validate enforces the finding contract: every text in every language, known severity, ID = folder.
func (r *Rule) Validate(engine, dir string) error {
	var errs []error
	if r.ID != dir {
		errs = append(errs, fmt.Errorf("id %q must match folder %q", r.ID, dir))
	}
	if r.Engine != engine {
		errs = append(errs, fmt.Errorf("engine %q must be %q", r.Engine, engine))
	}
	if _, ok := map[Severity]bool{Critical: true, High: true, Medium: true, Low: true}[r.Severity]; !ok {
		errs = append(errs, fmt.Errorf("unknown severity %q", r.Severity))
	}
	if r.Confidence != Confirmed && r.Confidence != Hint {
		errs = append(errs, fmt.Errorf("confidence must be confirmed or hint"))
	}
	required := map[string]Text{"title": r.Title, "impact": r.Impact, "next_action": r.NextAction}
	optional := map[string]Text{"caveat": r.Caveat, "saas_cta": r.SaaSCTA}
	for name, t := range required {
		if t == nil {
			errs = append(errs, fmt.Errorf("missing %s", name))
		}
	}
	for name, t := range mergeTexts(required, optional) {
		if t == nil {
			continue
		}
		for _, l := range Languages {
			if strings.TrimSpace(t[l]) == "" {
				errs = append(errs, fmt.Errorf("%s: missing language %s", name, l))
			}
		}
	}
	for _, m := range placeholder.FindAllStringSubmatch(r.SQL, -1) {
		if _, ok := r.Params[m[1]]; !ok {
			errs = append(errs, fmt.Errorf("query uses undefined param %q", m[1]))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("rule %s: %w", dir, err)
	}
	return nil
}

func mergeTexts(a, b map[string]Text) map[string]Text {
	out := map[string]Text{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// Render substitutes {{param}} with numeric params only, so no text can be injected into the SQL.
func (r *Rule) Render() string {
	return placeholder.ReplaceAllStringFunc(r.SQL, func(s string) string {
		name := placeholder.FindStringSubmatch(s)[1]
		return "(" + strconv.FormatFloat(r.Params[name], 'f', -1, 64) + ")" // parens: -1 must not become a -- comment
	})
}

func (r *Rule) applies(info ServerInfo) (bool, string) {
	if r.MinMajor > 0 && info.Major < r.MinMajor {
		return false, fmt.Sprintf("requires major version >= %d", r.MinMajor)
	}
	if r.MaxMajor > 0 && info.Major > r.MaxMajor {
		return false, fmt.Sprintf("requires major version <= %d", r.MaxMajor)
	}
	for _, p := range r.SkipOn {
		if p == info.Platform {
			return false, "not applicable on " + p
		}
	}
	return true, ""
}

// Run executes rules one at a time (never in parallel, to stay light on production).
// progress is called before each rule (res == nil) and after it (res set).
func Run(ctx context.Context, a Adapter, rules []*Rule, progress func(i, total int, r *Rule, res *Result)) []Result {
	info := a.Info()
	results := make([]Result, 0, len(rules))
	for i, r := range rules {
		if progress != nil {
			progress(i, len(rules), r, nil)
		}
		results = append(results, runOne(ctx, a, info, r))
		if progress != nil {
			progress(i, len(rules), r, &results[i])
		}
	}
	return results
}

func runOne(ctx context.Context, a Adapter, info ServerInfo, r *Rule) Result {
	res := Result{Rule: r, CollectedAt: time.Now().UTC()}
	if ok, why := r.applies(info); !ok {
		res.Status, res.Reason = StatusNotApplies, why
		return res
	}
	qctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	start := time.Now()
	var cols []string
	var rows [][]any
	var err error
	if r.Scope == "database" {
		cols, rows, res.Reason, err = queryEachDB(qctx, a, r.Render())
	} else {
		cols, rows, err = a.Query(qctx, r.Render())
	}
	res.Duration = time.Since(start).Round(time.Millisecond).String()
	switch {
	case errors.Is(qctx.Err(), context.DeadlineExceeded):
		res.Status, res.Confidence, res.Reason = StatusNoData, NoData, "timeout after "+r.Timeout.String()
	case err != nil:
		res.Status, res.Confidence, res.Reason = StatusNoData, NoData, err.Error()
	case len(rows) == 0:
		res.Status = StatusOK // Reason may still list skipped databases
	default:
		res.Status, res.Confidence = StatusFinding, r.Confidence
		res.Columns, res.Rows = splitConfidence(cols, rows, &res.Confidence)
	}
	return res
}

// splitConfidence drops an optional _confidence column; the strongest row wins (confirmed > hint).
func splitConfidence(cols []string, rows [][]any, conf *Confidence) ([]string, [][]any) {
	idx := -1
	for i, c := range cols {
		if c == "_confidence" {
			idx = i
		}
	}
	if idx < 0 {
		return cols, rows
	}
	*conf = Hint
	out := make([][]any, len(rows))
	for i, row := range rows {
		if fmt.Sprint(row[idx]) == string(Confirmed) {
			*conf = Confirmed
		}
		out[i] = append(append([]any{}, row[:idx]...), row[idx+1:]...)
	}
	return append(append([]string{}, cols[:idx]...), cols[idx+1:]...), out
}

// queryEachDB runs sql in every database the login can open and prefixes rows with a "database" column.
// Databases without access or that fail are listed in reason; it errors only if none could be read.
func queryEachDB(ctx context.Context, a Adapter, sql string) ([]string, [][]any, string, error) {
	open, noAccess, err := a.Databases(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	var cols []string
	var rows [][]any
	var skipped []string
	for _, db := range noAccess {
		skipped = append(skipped, db+" (no access)")
	}
	read := 0
	for _, db := range open {
		c, rs, err := a.QueryDB(ctx, db, sql)
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, "", ctx.Err()
			}
			skipped = append(skipped, db+" ("+err.Error()+")")
			continue
		}
		read++
		if cols == nil {
			cols = append([]string{"database"}, c...)
		}
		for _, row := range rs {
			rows = append(rows, append([]any{db}, row...))
		}
	}
	reason := ""
	if len(skipped) > 0 {
		reason = "skipped databases: " + strings.Join(skipped, "; ")
	}
	if read == 0 && len(skipped) > 0 {
		return nil, nil, "", errors.New(reason)
	}
	return cols, rows, reason, nil
}
