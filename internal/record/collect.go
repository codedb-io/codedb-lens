package record

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/codedb-io/codedb-lens/internal/engine"
)

// Intervals are fixed (Q6): fast collectors run every tick, slow ones every SlowEvery ticks.
const (
	FastInterval = 15 * time.Second
	SlowEvery    = 20 // 5 min
)

// Collector is one sampling query from rules/<engine>/_collect. Like the rules, the SQL is data.
type Collector struct {
	Name       string   `yaml:"name"`
	Every      string   `yaml:"every"` // fast | slow
	Mode       string   `yaml:"mode"`  // snapshot | delta
	Key        []string `yaml:"key"`   // delta: identifies a row between runs; empty = a single row
	Cumulative []string `yaml:"cumulative"`
	KeepTop    int      `yaml:"keep_top"` // keep only the N largest rows by OrderBy
	OrderBy    string   `yaml:"order_by"`
	Hash       string   `yaml:"hash"` // column with the query hash, for text lines
	Text       string   `yaml:"text"` // column with the query text: moved out of the rows into text lines
	SkipOn     []string `yaml:"skip_on"`

	SQL      string            `yaml:"-"`
	TextSQL  string            `yaml:"-"` // <name>.text.sql: texts for {{hashes}}, when the main query only has hashes
	Variants map[string]string `yaml:"-"` // <name>.<platform>.sql

	prev   map[string][]any
	prevAt time.Time
}

// Sample is what one collector run writes: rows (deltas, for delta collectors) and query texts by hash.
type Sample struct {
	Cols  []string
	Rows  [][]any
	Texts map[string]string
}

var collectorName = regexp.MustCompile(`^[a-z][a-z_]*$`)

// LoadCollectors reads <engine>/_collect/collect.yaml and its SQL files. Each call returns fresh collectors:
// delta state lives in them, so one set per recording.
func LoadCollectors(fsys fs.FS, eng string) ([]*Collector, error) {
	base := path.Join(eng, "_collect")
	raw, err := fs.ReadFile(fsys, path.Join(base, "collect.yaml"))
	if err != nil {
		return nil, err
	}
	var cs []*Collector
	if err := yaml.Unmarshal(raw, &cs); err != nil {
		return nil, fmt.Errorf("%s/collect.yaml: %w", base, err)
	}
	entries, err := fs.ReadDir(fsys, base)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, c := range cs {
		if seen[c.Name] {
			return nil, fmt.Errorf("%s: collector %q listed twice", base, c.Name)
		}
		seen[c.Name] = true
		for _, e := range entries {
			rest, ok := strings.CutPrefix(e.Name(), c.Name+".")
			if !ok || (rest != "sql" && !strings.HasSuffix(rest, ".sql")) {
				continue
			}
			q, err := fs.ReadFile(fsys, path.Join(base, e.Name()))
			if err != nil {
				return nil, err
			}
			switch v := strings.TrimSuffix(rest, ".sql"); {
			case rest == "sql":
				c.SQL = string(q)
			case v == "text":
				c.TextSQL = string(q)
			default:
				if c.Variants == nil {
					c.Variants = map[string]string{}
				}
				c.Variants[v] = string(q)
			}
		}
		if err := c.validate(); err != nil {
			return nil, fmt.Errorf("%s: collector %q: %w", base, c.Name, err)
		}
	}
	return cs, nil
}

func (c *Collector) validate() error {
	var errs []error
	if !collectorName.MatchString(c.Name) {
		errs = append(errs, errors.New("name must be lowercase letters and _"))
	}
	if c.Every != "fast" && c.Every != "slow" {
		errs = append(errs, fmt.Errorf("every %q must be fast or slow", c.Every))
	}
	switch c.Mode {
	case "snapshot":
	case "delta":
		if len(c.Cumulative) == 0 {
			errs = append(errs, errors.New("delta needs cumulative columns"))
		}
	default:
		errs = append(errs, fmt.Errorf("mode %q must be snapshot or delta", c.Mode))
	}
	if c.KeepTop > 0 && c.OrderBy == "" {
		errs = append(errs, errors.New("keep_top needs order_by"))
	}
	if (c.Text != "" || c.TextSQL != "") && c.Hash == "" {
		errs = append(errs, errors.New("query texts need a hash column"))
	}
	if c.TextSQL != "" && !strings.Contains(c.TextSQL, "{{hashes}}") {
		errs = append(errs, errors.New("text.sql must use {{hashes}}"))
	}
	if c.SQL == "" {
		errs = append(errs, fmt.Errorf("missing %s.sql", c.Name))
	}
	for _, q := range c.Queries() {
		if w := engine.WriteKeyword(q); w != "" {
			errs = append(errs, fmt.Errorf("a query uses %q; collectors must be read-only SELECTs", w))
		}
	}
	return errors.Join(errs...)
}

// Queries returns every SQL of the collector, for validation.
func (c *Collector) Queries() []string {
	qs := []string{c.SQL}
	if c.TextSQL != "" {
		qs = append(qs, c.TextSQL)
	}
	for _, v := range c.Variants {
		qs = append(qs, v)
	}
	return qs
}

// Query is the SQL that runs on platform.
func (c *Collector) Query(platform string) string {
	if v, ok := c.Variants[platform]; ok {
		return v
	}
	return c.SQL
}

func (c *Collector) Skips(platform string) bool { return slices.Contains(c.SkipOn, platform) }

// Slow collectors run on every SlowEvery-th tick, starting with the first.
func (c *Collector) Due(tick int) bool { return c.Every == "fast" || tick%SlowEvery == 0 }

var safeHash = regexp.MustCompile(`^(0x[0-9A-Fa-f]{1,64}|-?[0-9]{1,20})$`)

// TextQuery renders TextSQL for hashes. Hashes come from the server, but are checked before going into SQL.
func (c *Collector) TextQuery(hashes []string) (string, error) {
	for _, h := range hashes {
		if !safeHash.MatchString(h) {
			return "", fmt.Errorf("%s: unexpected query hash %q", c.Name, h)
		}
	}
	return strings.ReplaceAll(c.TextSQL, "{{hashes}}", strings.Join(hashes, ", ")), nil
}

// Texts reads the result of TextQuery: columns hash and text.
func Texts(cols []string, rows [][]any) map[string]string {
	hi, ti := slices.Index(cols, "hash"), slices.Index(cols, "text")
	out := map[string]string{}
	if hi < 0 || ti < 0 {
		return out
	}
	for _, r := range rows {
		if h, t := str(r[hi]), str(r[ti]); h != "" && t != "" {
			out[h] = truncate(t, maxText)
		}
	}
	return out
}

// Process turns the rows of one run into a sample. A delta collector returns ok = false on its first run, which
// only sets the baseline.
func (c *Collector) Process(now time.Time, cols []string, rows [][]any) (s Sample, ok bool, err error) {
	at := func(name string) (int, error) {
		if i := slices.Index(cols, name); i >= 0 {
			return i, nil
		}
		return 0, fmt.Errorf("%s: column %q not in the query result", c.Name, name)
	}
	s.Texts = map[string]string{}

	if c.Text != "" { // move texts out before deltas, so the baseline never holds them
		ti, err := at(c.Text)
		if err != nil {
			return s, false, err
		}
		hi, err := at(c.Hash)
		if err != nil {
			return s, false, err
		}
		for i, r := range rows {
			if h, t := str(r[hi]), str(r[ti]); h != "" && t != "" {
				s.Texts[h] = truncate(t, maxText)
			}
			rows[i] = slices.Delete(slices.Clone(r), ti, ti+1)
		}
		cols = slices.Delete(slices.Clone(cols), ti, ti+1)
	}

	if c.Mode == "delta" {
		keys := make([]int, len(c.Key))
		for i, k := range c.Key {
			if keys[i], err = at(k); err != nil {
				return s, false, err
			}
		}
		cum := make([]int, len(c.Cumulative))
		for i, k := range c.Cumulative {
			if cum[i], err = at(k); err != nil {
				return s, false, err
			}
		}
		first := c.prev == nil
		if first {
			c.prev = map[string][]any{}
		}
		interval := math.Round(now.Sub(c.prevAt).Seconds()*10) / 10
		var out [][]any
		for _, r := range rows {
			k := key(r, keys)
			p, seen := c.prev[k]
			// ponytail: the baseline keeps rows that disappear (a query not run this cycle), so they get a true
			// delta when they come back. A key never seen before is only a baseline: its counters may hold
			// hours of history, which would show up as a false peak. Ceiling: a query's first run is not counted.
			c.prev[k] = r
			if first || !seen {
				continue
			}
			if d, changed := delta(r, p, cum); changed {
				out = append(out, append(d, interval))
			}
		}
		c.prevAt = now
		if first {
			return s, false, nil
		}
		cols, rows = append(slices.Clone(cols), "interval_s"), out
	}

	if c.KeepTop > 0 {
		oi, err := at(c.OrderBy)
		if err != nil {
			return s, false, err
		}
		sort.SliceStable(rows, func(i, j int) bool {
			a, _ := engine.Number(rows[i][oi])
			b, _ := engine.Number(rows[j][oi])
			return a > b
		})
		rows = rows[:min(len(rows), c.KeepTop)]
	}

	if c.Hash != "" && len(s.Texts) > 0 { // texts only for rows that were kept
		hi, err := at(c.Hash)
		if err != nil {
			return s, false, err
		}
		kept := map[string]string{}
		for _, r := range rows {
			if t, ok := s.Texts[str(r[hi])]; ok {
				kept[str(r[hi])] = t
			}
		}
		s.Texts = kept
	}
	s.Cols, s.Rows = cols, rows
	return s, true, nil
}

// delta subtracts the cumulative columns. A counter that went down (restart, stats cleared, plan evicted) is not a
// delta: the row is skipped and the new value is the next baseline. Rows where nothing changed are skipped too.
func delta(cur, prev []any, cum []int) ([]any, bool) {
	d := slices.Clone(cur)
	changed := false
	for _, i := range cum {
		a, okA := engine.Number(cur[i])
		b, okB := engine.Number(prev[i])
		if !okA || !okB {
			d[i] = nil
			continue
		}
		if a < b {
			return nil, false
		}
		if a != b {
			changed = true
		}
		d[i] = a - b
	}
	return d, changed
}

func key(r []any, idx []int) string {
	var b strings.Builder
	for _, i := range idx {
		b.WriteString(str(r[i]))
		b.WriteByte(0)
	}
	return b.String()
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
