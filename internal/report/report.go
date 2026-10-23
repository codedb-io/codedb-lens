// Package report writes the self-contained HTML report (both languages embedded) and the JSON report.
package report

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/saas"
)

//go:embed report.html.tmpl
var tmplText string

//go:embed tokens.css
var Tokens string // design tokens, shared with the GUI

//go:embed style.css
var style string

//go:embed icon.png
var icon []byte

type Report struct {
	Tool        string    `json:"tool"`
	ToolVersion string    `json:"tool_version"`
	GeneratedAt time.Time `json:"generated_at"`
	// CatalogVersion fingerprints the rules and weights used (engine.CatalogVersion); empty before v0.5.
	CatalogVersion string            `json:"catalog_version,omitempty"`
	Duration       string            `json:"duration,omitempty"` // whole run, connection included
	Lang           string            `json:"lang"`
	Server         engine.ServerInfo `json:"server"`
	Score          engine.Score      `json:"score"`
	Results        []engine.Result   `json:"results"`
	// Evolution compares with the previous run of the same instance in the same folder; set by SetEvolution.
	Evolution *Evolution `json:"evolution,omitempty"`
	// Redacted marks a report whose names were masked by Redact; it is saved as <timestamp>.redacted.*.
	Redacted bool `json:"redacted,omitempty"`
}

// Evolution feeds the executive summary. State is "first" (no previous run), "locked" (previous run exists
// but no valid free key, so no comparison) or "compared".
type Evolution struct {
	State        string    `json:"state"`
	PrevAt       time.Time `json:"previous_at,omitzero"`
	PrevLetter   string    `json:"previous_letter,omitempty"`
	PrevCoverage float64   `json:"previous_coverage,omitempty"`
	New          int       `json:"new,omitempty"`
	Resolved     int       `json:"resolved,omitempty"`
}

var severityOrder = map[engine.Severity]int{engine.Critical: 0, engine.High: 1, engine.Medium: 2, engine.Low: 3}
var statusOrder = map[engine.Status]int{engine.StatusFinding: 0, engine.StatusNoData: 1, engine.StatusOK: 2, engine.StatusNotApplies: 3}

func New(version, lang string, info engine.ServerInfo, score engine.Score, results []engine.Result) *Report {
	sorted := append([]engine.Result{}, results...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if statusOrder[a.Status] != statusOrder[b.Status] {
			return statusOrder[a.Status] < statusOrder[b.Status]
		}
		return severityOrder[a.Rule.Severity] < severityOrder[b.Rule.Severity]
	})
	return &Report{Tool: "codeDB Lens", ToolVersion: version, GeneratedAt: time.Now(), Lang: lang,
		Server: info, Score: score, Results: sorted}
}

func (r *Report) Count(s engine.Status) int {
	n := 0
	for _, x := range r.Results {
		if x.Status == s {
			n++
		}
	}
	return n
}

// Findings returns findings in report order (severity first).
func (r *Report) Findings() []engine.Result {
	var out []engine.Result
	for _, x := range r.Results {
		if x.Status == engine.StatusFinding {
			out = append(out, x)
		}
	}
	return out
}

// Top returns the n most urgent findings: severity, then Confirmed before Hint, then rule ID.
func (r *Report) Top(n int) []engine.Result {
	f := r.Findings()
	sort.SliceStable(f, func(i, j int) bool {
		a, b := f[i], f[j]
		if sa, sb := severityOrder[a.Rule.Severity], severityOrder[b.Rule.Severity]; sa != sb {
			return sa < sb
		}
		if ca, cb := a.Confidence == engine.Confirmed, b.Confidence == engine.Confirmed; ca != cb {
			return ca
		}
		return a.Rule.ID < b.Rule.ID
	})
	return f[:min(n, len(f))]
}

// Highest is the severity of the most urgent finding, "" when there are none.
func (r *Report) Highest() engine.Severity {
	if t := r.Top(1); len(t) > 0 {
		return t[0].Rule.Severity
	}
	return ""
}

// SetEvolution fills Evolution from the newest earlier JSON report of the same instance in dir.
// allowed = the free key is valid (the same gate as compare); without it only the existence of a previous run is told.
func (r *Report) SetEvolution(dir string, allowed bool) {
	prev := previous(dir, r)
	switch {
	case prev == nil:
		r.Evolution = &Evolution{State: "first"}
	case !allowed:
		r.Evolution = &Evolution{State: "locked"}
	default:
		c, err := Compare(r.ToolVersion, r.Lang, prev, r, "", "")
		if err != nil { // unreachable: previous() already matched the instance
			return
		}
		r.Evolution = &Evolution{State: "compared", PrevAt: prev.GeneratedAt, PrevLetter: prev.Score.Letter,
			PrevCoverage: prev.Score.Coverage, New: len(c.New), Resolved: len(c.Resolved)}
	}
}

// previous is the newest report in dir from before cur, same instance (and same masking: tokens never match real names). ponytail: reads at most the 20 newest files.
func previous(dir string, cur *Report) *Report {
	for i, f := range jsonFiles(dir) {
		if i == 20 {
			break
		}
		if p, err := Load(f); err == nil && SameInstance(p.Server, cur.Server) && p.Redacted == cur.Redacted && p.GeneratedAt.Before(cur.GeneratedAt) {
			return p
		}
	}
	return nil
}

// jsonFiles lists the run reports in dir (not comparisons), newest first.
func jsonFiles(dir string) []string {
	all, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	sort.Sort(sort.Reverse(sort.StringSlice(all))) // names are timestamps
	var files []string
	for _, f := range all {
		if !strings.HasPrefix(filepath.Base(f), "compare_") {
			files = append(files, f)
		}
	}
	return files
}

// CountSev counts findings with any of the given severities.
func (r *Report) CountSev(sev ...string) int {
	n := 0
	for _, x := range r.Findings() {
		for _, s := range sev {
			if string(x.Rule.Severity) == s {
				n++
			}
		}
	}
	return n
}

// both renders a message in every language; CSS shows only the active one.
func both(get func(lang string) string) template.HTML {
	var b strings.Builder
	for _, l := range engine.Languages {
		fmt.Fprintf(&b, `<span lang="%s">%s</span>`, l, html.EscapeString(get(l)))
	}
	return template.HTML(b.String())
}

var funcs = template.FuncMap{
	"t": func(key string) template.HTML { return both(func(l string) string { return i18n.T(l, key) }) },
	"tf": func(key string, args ...any) template.HTML {
		return both(func(l string) string { return fmt.Sprintf(i18n.T(l, key), args...) })
	},
	"tx": func(t engine.Text) template.HTML { return both(func(l string) string { return t[l] }) },
	// txc is tx with `inline code` rendered as <code> (escaped first, so the tags are the only markup).
	"txc": func(t engine.Text) template.HTML {
		var b strings.Builder
		for _, l := range engine.Languages {
			parts := strings.Split(html.EscapeString(t[l]), "`")
			for i := 1; i < len(parts); i += 2 {
				parts[i] = "<code>" + parts[i] + "</code>"
			}
			fmt.Fprintf(&b, `<span lang="%s">%s</span>`, l, strings.Join(parts, ""))
		}
		return template.HTML(b.String())
	},
	"landing":  func(campaign string) string { return saas.Link(saas.Landing, "report", campaign) },
	"register": func(campaign string) string { return saas.Link(saas.Register, "report", campaign) },
	"sumlink":  func(campaign string) string { return saas.Link(saas.Register, "summary", campaign) },
	"docpt":    func(u string) string { return strings.Replace(u, "/rules/", "/pt-BR/rules/", 1) },
	"pct":      func(f float64) string { return fmt.Sprintf("%.0f%%", f*100) },
	"str": func(v any) string {
		if v == nil {
			return "NULL"
		}
		return fmt.Sprint(v)
	},
	"lower": strings.ToLower,
	"date":  func(t time.Time) string { return t.Format("2006-01-02 15:04") },
	// localtime shows a UTC timestamp in this machine's zone, like the report date; older reports kept the raw text.
	"localtime": func(s string) string {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.Local().Format("2006-01-02 15:04:05")
		}
		return s
	},
	"css":  func() template.CSS { return template.CSS(Tokens + style) },
	"add1": func(i int) int { return i + 1 },
	// Evidence columns named "meaning@en" / "meaning@pt-BR" show only in their language.
	"colname": func(c string) string { name, _, _ := strings.Cut(c, "@"); return name },
	"collang": func(c string) template.HTMLAttr {
		if _, l, ok := strings.Cut(c, "@"); ok {
			return template.HTMLAttr(` lang="` + html.EscapeString(l) + `"`)
		}
		return ""
	},
	"engineName": func(e string) string {
		if e == "pg" {
			return "PostgreSQL"
		}
		return "SQL Server"
	},
	"icon": func() template.URL {
		return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(icon))
	},
}

var tmpl = template.Must(template.New("report").Funcs(funcs).Parse(tmplText))

func (r *Report) HTML(w io.Writer) error { return tmpl.Execute(w, r) }

func (r *Report) JSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// Save writes <dir>/<YYYY-MM-DD_HHMMSS>.html and .json (.redacted.html/.json when masked) and returns the HTML path.
func (r *Report) Save(dir string) (string, error) {
	base := filepath.Join(dir, r.GeneratedAt.Format("2006-01-02_150405")) // seconds: two runs in a minute must not overwrite
	if r.Redacted {
		base += ".redacted"
	}
	return base + ".html", writeAll(dir, base, map[string]func(io.Writer) error{".html": r.HTML, ".json": r.JSON})
}

func writeAll(dir, base string, files map[string]func(io.Writer) error) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for ext, write := range files {
		f, err := os.Create(base + ext)
		if err != nil {
			return err
		}
		err = write(f)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// DefaultDir is Documents/codeDB Lens/reports/<profile> on Windows, ~/codedb-lens/reports/<profile> elsewhere.
func DefaultDir(profile string) string {
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, "codedb-lens", "reports")
	// ponytail: ~/Documents, ignores a Documents folder moved by OneDrive; use SHGetKnownFolderPath if users hit it.
	if os.PathSeparator == '\\' {
		root = filepath.Join(home, "Documents", "codeDB Lens", "reports")
	}
	return filepath.Join(root, safeName(firstNonEmpty(profile, "default")))
}

func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*,`, r) || r < 32 {
			return '_'
		}
		return r
	}, s)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// Summary is one past run, read back from its JSON for the app's history and the sidebar badge.
type Summary struct {
	When     time.Time `json:"when"`
	Letter   string    `json:"letter"`
	Value    float64   `json:"value"`
	Coverage float64   `json:"coverage"`
	Findings int       `json:"findings"`
	Path     string    `json:"path"` // HTML report
}

// History lists the newest runs in dir (at most n), newest first.
func History(dir string, n int) []Summary {
	var out []Summary
	for _, f := range jsonFiles(dir) {
		if len(out) == n {
			break
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var r Report
		if json.Unmarshal(raw, &r) != nil {
			continue
		}
		out = append(out, Summary{When: r.GeneratedAt, Letter: r.Score.Letter, Value: r.Score.Value, Coverage: r.Score.Coverage,
			Findings: r.Count(engine.StatusFinding), Path: strings.TrimSuffix(f, ".json") + ".html"})
	}
	return out
}
