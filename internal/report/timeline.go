package report

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"math"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/record"
	"github.com/codedb-io/codedb-lens/rules"
)

//go:embed timeline.html.tmpl
var timelineText string

// Timeline is the report of a recording (codedb-lens report): activity over time, the series rules' findings and
// the top queries of the period. It reads only the recording folder; nothing connects to the server.
type Timeline struct {
	Tool        string          `json:"tool"`
	ToolVersion string          `json:"tool_version"`
	GeneratedAt time.Time       `json:"generated_at"`
	Lang        string          `json:"lang"`
	Recording   record.Meta     `json:"recording"`
	From        time.Time       `json:"from"`
	To          time.Time       `json:"to"`
	Cycles      int             `json:"cycles"`          // fast cycles with at least one sample
	Expected    int             `json:"expected_cycles"` // fast cycles the period should have
	Coverage    float64         `json:"coverage"`        // Cycles / Expected: the coverage seal of a recording
	Gaps        []Span          `json:"gaps,omitempty"`  // stretches without any sample
	Charts      []Chart         `json:"charts"`
	Results     []engine.Result `json:"results"` // series rules
	Top         Table           `json:"top_queries"`
	Redacted    bool            `json:"redacted,omitempty"`

	interval time.Duration
	infoN    int
}

type Span struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type Point struct {
	T time.Time `json:"t"`
	V float64   `json:"v"`
}

type Chart struct {
	Key      string  `json:"key"` // i18n title; tl_info_<key minus tl_> explains it
	Source   string  `json:"source"`
	Points   []Point `json:"points"`
	Max      float64 `json:"max"`
	Episodes []Span  `json:"episodes,omitempty"` // findings on the same collector
}

type Table struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// chartDef turns one sample of a collector into one value. ponytail: charts are presentation, kept in Go; the
// verdicts stay in the series rules (data).
type chartDef struct {
	key, source string
	value       func(s engine.Sample) (float64, bool)
}

func col(s engine.Sample, name string) int { return slices.Index(s.Cols, name) }

func count(where func(s engine.Sample, r []any) bool) func(engine.Sample) (float64, bool) {
	return func(s engine.Sample) (float64, bool) {
		n := 0
		for _, r := range s.Rows {
			if where == nil || where(s, r) {
				n++
			}
		}
		return float64(n), true
	}
}

// perSecond sums a delta column and divides by the sample's interval. No rows = nothing moved = 0.
func perSecond(name string) func(engine.Sample) (float64, bool) {
	return func(s engine.Sample) (float64, bool) {
		ci, ii := col(s, name), col(s, "interval_s")
		if ci < 0 || ii < 0 {
			return 0, len(s.Rows) == 0
		}
		sum, iv := 0.0, 0.0
		for _, r := range s.Rows {
			v, _ := engine.Number(r[ci])
			sum += v
			iv, _ = engine.Number(r[ii])
		}
		if iv <= 0 {
			return 0, true
		}
		return sum / iv, true
	}
}

func value(name string) func(engine.Sample) (float64, bool) {
	return func(s engine.Sample) (float64, bool) {
		if ci := col(s, name); ci >= 0 && len(s.Rows) > 0 {
			return engine.Number(s.Rows[0][ci])
		}
		return 0, false
	}
}

func pctOf(name, of string) func(engine.Sample) (float64, bool) {
	return func(s engine.Sample) (float64, bool) {
		a, okA := value(name)(s)
		b, okB := value(of)(s)
		if !okA || !okB || b <= 0 {
			return 0, false
		}
		return 100 * a / b, true
	}
}

func textIs(name, want string) func(engine.Sample, []any) bool {
	return func(s engine.Sample, r []any) bool { i := col(s, name); return i >= 0 && fmt.Sprint(r[i]) == want }
}

var charts = map[string][]chartDef{
	"mssql": {
		{"tl_ch_requests", "requests", count(nil)},
		{"tl_ch_blocked", "locks", count(nil)},
		{"tl_ch_waits", "waits", perSecond("wait_ms")},
		{"tl_ch_batch", "counters", perSecond("batch_requests")},
		{"tl_ch_ple", "counters", value("ple_s")},
		{"tl_ch_workers", "workers", pctOf("workers", "max_workers")},
	},
	"pg": {
		{"tl_ch_active", "activity", count(textIs("state", "active"))},
		{"tl_ch_waiting", "activity", count(func(s engine.Sample, r []any) bool {
			i := col(s, "wait_event_type")
			w := ""
			if i >= 0 && r[i] != nil {
				w = fmt.Sprint(r[i])
			}
			return textIs("state", "active")(s, r) && w != "" && w != "Client"
		})},
		{"tl_ch_blocked", "locks", count(nil)},
		{"tl_ch_commits", "dbstats", perSecond("commits")},
		{"tl_ch_conn", "connections", pctOf("connections", "max_connections")},
	},
}

const topQueries = 20

// ownMarker is in every collector query (rules/<engine>/_collect), so the report can leave the Lens out of the top.
const ownMarker = "/* codeDB Lens */"

// BuildTimeline reads a recording and evaluates it between from and to (zero = the whole recording).
func BuildTimeline(dir string, from, to time.Time, version, lang string) (*Timeline, error) {
	meta, err := record.ReadMeta(dir)
	if err != nil {
		return nil, err
	}
	eng := meta.Server.Engine
	collectors, err := record.LoadCollectors(rules.FS, eng)
	if err != nil {
		return nil, err
	}
	series, err := engine.LoadSeries(rules.FS, eng)
	if err != nil {
		return nil, err
	}
	interval, err := time.ParseDuration(meta.Intervals["fast"])
	if err != nil || interval <= 0 {
		interval = record.FastInterval
	}

	samples := map[string][]engine.Sample{}
	texts := map[string]string{}
	cycles := map[int64]bool{}
	var first, last time.Time
	err = record.ReadAll(dir, from, to, func(l record.Line) error {
		if first.IsZero() {
			first = l.T
		}
		last = l.T
		switch l.K {
		case record.KindText:
			texts[l.H] = l.Q
		case record.KindGap:
			names := l.C
			if len(names) == 0 { // connection down: every collector
				for _, c := range collectors {
					names = append(names, c.Name)
				}
			}
			for _, n := range names {
				samples[n] = append(samples[n], engine.Sample{T: l.T, Gap: true})
			}
		default:
			samples[l.K] = append(samples[l.K], engine.Sample{T: l.T, Cols: l.C, Rows: l.R})
			cycles[l.T.Unix()] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	t := &Timeline{Tool: "codeDB Lens", ToolVersion: version, GeneratedAt: time.Now(), Lang: lang, Recording: meta,
		interval: interval}
	t.From, t.To = periodBounds(meta, from, to, first, last, interval)
	t.Cycles = len(cycles)
	t.Expected = max(1, int(math.Round(float64(t.To.Sub(t.From))/float64(interval))))
	t.Coverage = math.Min(1, float64(t.Cycles)/float64(t.Expected))
	t.Gaps = gaps(cycles, t.From, t.To, interval)

	for _, r := range series {
		var res engine.Result
		if r.Skips(meta.Server.Platform) {
			res = engine.Result{Rule: r, Status: engine.StatusNotApplies, Reason: meta.Server.Platform}
		} else {
			res = engine.EvaluateSeries(r, samples[r.Source])
		}
		addQueryText(&res, texts)
		t.Results = append(t.Results, res)
	}
	sort.SliceStable(t.Results, func(i, j int) bool {
		a, b := t.Results[i], t.Results[j]
		if statusOrder[a.Status] != statusOrder[b.Status] {
			return statusOrder[a.Status] < statusOrder[b.Status]
		}
		return severityOrder[a.Rule.Severity] < severityOrder[b.Rule.Severity]
	})

	for _, d := range charts[eng] {
		c := Chart{Key: d.key, Source: d.source}
		for _, s := range samples[d.source] {
			if s.Gap {
				continue
			}
			if v, ok := d.value(s); ok {
				c.Points = append(c.Points, Point{s.T, math.Round(v*10) / 10})
				c.Max = math.Max(c.Max, v)
			}
		}
		for _, res := range t.Results {
			if res.Status == engine.StatusFinding && res.Rule.Source == d.source {
				for _, row := range res.Rows {
					a, _ := time.Parse(time.RFC3339, fmt.Sprint(row[0]))
					b, _ := time.Parse(time.RFC3339, fmt.Sprint(row[1]))
					c.Episodes = append(c.Episodes, Span{a, b})
				}
			}
		}
		t.Charts = append(t.Charts, c)
	}

	for _, c := range collectors {
		if c.TextSQL != "" {
			t.Top = topTable(c, samples[c.Name], texts)
		}
	}
	return t, nil
}

// periodBounds: the asked period, clipped to the recording; a recording without stopped_at ends at its last line.
func periodBounds(m record.Meta, from, to, first, last time.Time, interval time.Duration) (time.Time, time.Time) {
	start, end := m.StartedAt, m.StoppedAt
	if end.IsZero() {
		end = last.Add(interval)
		if last.IsZero() {
			end = start
		}
	}
	if !from.IsZero() && from.After(start) {
		start = from
	}
	if !to.IsZero() && to.Before(end) {
		end = to
	}
	if !end.After(start) {
		end = start.Add(interval)
	}
	return start, end
}

// gaps are the stretches longer than two cycles without any sample, including at the start and the end.
func gaps(cycles map[int64]bool, from, to time.Time, interval time.Duration) []Span {
	ts := make([]int64, 0, len(cycles))
	for k := range cycles {
		ts = append(ts, k)
	}
	slices.Sort(ts)
	var out []Span
	prev := from
	for _, u := range append(ts, to.Unix()) {
		t := time.Unix(u, 0).UTC()
		if t.Sub(prev) > 2*interval {
			out = append(out, Span{prev, t})
		}
		prev = t
	}
	return out
}

// addQueryText puts the text of query_hash next to the evidence, so the report says which query it was.
func addQueryText(res *engine.Result, texts map[string]string) {
	hi := slices.Index(res.Columns, "query_hash")
	if hi < 0 || slices.Contains(res.Columns, "query") {
		return
	}
	res.Columns = append(res.Columns, "query")
	for i, r := range res.Rows {
		res.Rows[i] = append(r, texts[fmt.Sprint(r[hi])])
	}
}

// topTable sums the deltas of the slow collector per query over the period and keeps the heaviest.
func topTable(c *record.Collector, samples []engine.Sample, texts map[string]string) Table {
	type agg struct {
		hash string
		sums []float64
	}
	byHash := map[string]*agg{}
	for _, s := range samples {
		hi := col(s, c.Hash)
		if s.Gap || hi < 0 {
			continue
		}
		idx := make([]int, len(c.Cumulative))
		for i, name := range c.Cumulative {
			idx[i] = col(s, name)
		}
		for _, r := range s.Rows {
			h := fmt.Sprint(r[hi])
			if strings.Contains(texts[h], ownMarker) {
				continue // the recorder's own queries
			}
			a := byHash[h]
			if a == nil {
				a = &agg{hash: h, sums: make([]float64, len(idx))}
				byHash[h] = a
			}
			for i, ci := range idx {
				if ci >= 0 {
					v, _ := engine.Number(r[ci])
					a.sums[i] += v
				}
			}
		}
	}
	order := max(slices.Index(c.Cumulative, c.OrderBy), 0)
	all := make([]*agg, 0, len(byHash))
	for _, a := range byHash {
		all = append(all, a)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].sums[order] > all[j].sums[order] })
	t := Table{Columns: append([]string{"query", "query_hash"}, c.Cumulative...)}
	for _, a := range all[:min(len(all), topQueries)] {
		row := []any{texts[a.hash], a.hash}
		for _, v := range a.sums {
			row = append(row, v)
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// Redact masks the timeline like a check report: names become tokens, query texts go.
func (t *Timeline) Redact(salt []byte) {
	r := &Report{Server: t.Recording.Server, Results: t.Results}
	r.Redact(salt)
	t.Recording.Server.Host = r.Server.Host
	for _, row := range t.Top.Rows {
		if row[0] != "" {
			row[0] = redactedText
		}
	}
	t.Redacted = true
}

// Info renders the "i" button and its popover (native HTML popover: click, touch, keyboard, Esc; no script).
func (t *Timeline) Info(key string) template.HTML {
	if !i18n.Has(key) {
		return ""
	}
	t.infoN++
	id := fmt.Sprintf("info-%d", t.infoN)
	label := strings.ReplaceAll(string(both(func(l string) string { return i18n.T(l, "tl_what") })), "<span ", `<span class="sr" `)
	return template.HTML(fmt.Sprintf(`<button type="button" class="info" popovertarget="%s"><span aria-hidden="true">i</span>%s</button><div id="%s" popover class="pop">%s</div>`,
		id, label, id, both(func(l string) string { return i18n.T(l, key) })))
}

// ColInfo is Info for a table column, when its explanation exists.
func (t *Timeline) ColInfo(column string) template.HTML { return t.Info("tl_col_" + column) }

func (t *Timeline) Findings() []engine.Result {
	var out []engine.Result
	for _, r := range t.Results {
		if r.Status == engine.StatusFinding {
			out = append(out, r)
		}
	}
	return out
}

func (t *Timeline) Count(s engine.Status) int {
	n := 0
	for _, r := range t.Results {
		if r.Status == s {
			n++
		}
	}
	return n
}

// ChartSVG draws a chart over the period: grey bands where nothing was recorded, red bands for episodes, and the
// line broken wherever samples are more than two cycles apart.
func (t *Timeline) ChartSVG(c Chart) template.HTML {
	const w, h, pad = 1000.0, 120.0, 6.0
	span := float64(t.To.Sub(t.From))
	x := func(tm time.Time) float64 { return math.Max(0, math.Min(w, float64(tm.Sub(t.From))/span*w)) }
	top := c.Max
	if top <= 0 {
		top = 1
	}
	y := func(v float64) float64 { return h - pad - v/top*(h-2*pad) }
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %.0f %.0f" preserveAspectRatio="none" role="img" aria-label="%s">`, w, h,
		html.EscapeString(i18n.T(t.Lang, c.Key)))
	band := func(s Span, class string) {
		fmt.Fprintf(&b, `<rect class="%s" x="%.1f" y="0" width="%.1f" height="%.0f"/>`, class, x(s.From), math.Max(2, x(s.To)-x(s.From)), h)
	}
	for _, g := range t.Gaps {
		band(g, "gap")
	}
	for _, e := range c.Episodes {
		band(e, "ep")
	}
	var line []string
	flush := func() {
		if len(line) == 1 { // a lone point still shows
			line = append(line, line[0])
		}
		if len(line) > 0 {
			fmt.Fprintf(&b, `<polyline points="%s"/>`, strings.Join(line, " "))
		}
		line = nil
	}
	for i, p := range c.Points {
		if i > 0 && p.T.Sub(c.Points[i-1].T) > 2*t.interval {
			flush()
		}
		line = append(line, fmt.Sprintf("%.1f,%.1f", x(p.T), y(p.V)))
	}
	flush()
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// Ticks are the time labels under the charts: start, three in between, end (local time).
func (t *Timeline) Ticks() []string {
	var out []string
	layout := "15:04"
	if t.To.Sub(t.From) > 20*time.Hour || t.To.Local().YearDay() != t.From.Local().YearDay() {
		layout = "02/01 15:04"
	}
	for i := 0; i <= 4; i++ {
		out = append(out, t.From.Add(t.To.Sub(t.From)*time.Duration(i)/4).Local().Format(layout))
	}
	return out
}

func (t *Timeline) HTML(w io.Writer) error {
	t.infoN = 0
	return timelineTmpl.Execute(w, t)
}

func (t *Timeline) JSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(t)
}

// Save writes report[-<from>_<to>][.redacted].html and .json into dir and returns the HTML path.
func (t *Timeline) Save(dir string, partial bool) (string, error) {
	base := "report"
	if partial {
		base += "-" + t.From.Local().Format("20060102-1504") + "_" + t.To.Local().Format("20060102-1504")
	}
	if t.Redacted {
		base += ".redacted"
	}
	base = filepath.Join(dir, base)
	return base + ".html", writeAll(dir, base, map[string]func(io.Writer) error{".html": t.HTML, ".json": t.JSON})
}

// num shows evidence numbers without exponents: 1234567.8 instead of 1.2345678e+06.
func num(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e15 {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', 1, 64)
	}
	return fmt.Sprint(v)
}

var timelineTmpl = template.Must(template.New("timeline").Funcs(funcs).Funcs(template.FuncMap{
	"num":    num,
	"ltime":  func(tm time.Time) string { return tm.Local().Format("2006-01-02 15:04") },
	"trimtl": func(k string) string { return strings.TrimPrefix(k, "tl_") },
	"dur":    func(a, b time.Time) string { return b.Sub(a).Round(time.Minute).String() },
	"peak":   func(v float64) string { return num(math.Round(v*10) / 10) },
	"short": func(v any) string {
		s := fmt.Sprint(v)
		if len([]rune(s)) > 160 {
			return string([]rune(s)[:160]) + "…"
		}
		return s
	},
}).Parse(timelineText))
