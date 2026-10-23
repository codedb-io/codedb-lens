package engine

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Series rules (kind: series) judge a recording instead of a snapshot. They read the samples of one record collector
// (source), keep the groups of rows where the threshold holds, and turn consecutive samples into episodes.
// Like the snapshot rules they are data: the YAML says what to measure, this file only counts.

const KindSeries = "series"

// Cond is a row filter (where: column op value, compared as text) or the threshold (op param, compared as a number).
type Cond struct {
	Column string `yaml:"column" json:"column,omitempty"`
	Op     string `yaml:"op" json:"op"`
	Value  string `yaml:"value" json:"value,omitempty"`
	Param  string `yaml:"param" json:"param,omitempty"`
}

// Sample is one run of a collector in a recording. Gap marks a run that failed or a lost connection: it ends every
// open episode, so an outage never joins two episodes into one.
type Sample struct {
	T    time.Time
	Cols []string
	Rows [][]any
	Gap  bool
}

var (
	numOps  = map[string]func(a, b float64) bool{">=": func(a, b float64) bool { return a >= b }, ">": func(a, b float64) bool { return a > b }, "<=": func(a, b float64) bool { return a <= b }, "<": func(a, b float64) bool { return a < b }}
	textOps = map[string]func(a, b string) bool{"=": func(a, b string) bool { return a == b }, "!=": func(a, b string) bool { return a != b }}
)

func (r *Rule) validateSeries() []error {
	var errs []error
	if r.Source == "" {
		errs = append(errs, errors.New("series rule needs a source collector"))
	}
	if r.Measure == "" {
		errs = append(errs, errors.New("series rule needs a measure (a column or count)"))
	}
	if r.Measure == "count" && (r.PerS || r.Of != "") {
		errs = append(errs, errors.New("count cannot be per_s or of"))
	}
	if numOps[r.Threshold.Op] == nil {
		errs = append(errs, fmt.Errorf("threshold op %q must be >=, >, <= or <", r.Threshold.Op))
	}
	if _, ok := r.Params[r.Threshold.Param]; !ok {
		errs = append(errs, fmt.Errorf("threshold uses undefined param %q", r.Threshold.Param))
	}
	for _, w := range r.Where {
		if textOps[w.Op] == nil || w.Column == "" {
			errs = append(errs, fmt.Errorf("where %q %q: needs a column and = or !=", w.Column, w.Op))
		}
	}
	if r.Sustain < 0 {
		errs = append(errs, errors.New("sustain must be 1 or more"))
	}
	return errs
}

type episode struct {
	start, end time.Time
	n          int
	peak       float64
	cols       []string
	row        []any
}

// EvaluateSeries runs a series rule over the samples of its source, in time order. No samples = no data, never OK.
func EvaluateSeries(r *Rule, samples []Sample) Result {
	res := Result{Rule: r, CollectedAt: time.Now().UTC()}
	sustain := max(r.Sustain, 1)
	open := map[string]*episode{}
	var done []*episode
	closeExcept := func(keep map[string]bool) {
		for k, e := range open {
			if !keep[k] {
				if e.n >= sustain {
					done = append(done, e)
				}
				delete(open, k)
			}
		}
	}
	seen := false
	for _, s := range samples {
		if s.Gap {
			closeExcept(nil)
			continue
		}
		seen = true
		hits, err := r.measure(s)
		if err != nil {
			res.Status, res.Confidence, res.Reason = StatusNoData, NoData, err.Error()
			return res
		}
		keep := map[string]bool{}
		for k, h := range hits {
			keep[k] = true
			e := open[k]
			if e == nil {
				e = &episode{start: s.T}
				open[k] = e
			}
			e.end = s.T
			if e.n++; e.n == 1 || h.v > e.peak {
				e.peak, e.cols, e.row = h.v, s.Cols, h.row
			}
		}
		closeExcept(keep)
	}
	closeExcept(nil)
	switch {
	case !seen:
		res.Status, res.Confidence, res.Reason = StatusNoData, NoData, "no samples from collector "+r.Source+" in this period"
		return res
	case len(done) == 0:
		res.Status = StatusOK
		return res
	}
	sort.Slice(done, func(i, j int) bool { return done[i].start.Before(done[j].start) })
	res.Status, res.Confidence = StatusFinding, r.Confidence
	res.Columns = append([]string{"start", "end", "duration_s", "samples", "peak"}, done[0].cols...)
	for _, e := range done {
		row := []any{e.start.UTC().Format(time.RFC3339), e.end.UTC().Format(time.RFC3339), e.end.Sub(e.start).Seconds(),
			e.n, math.Round(e.peak*10) / 10}
		res.Rows = append(res.Rows, append(row, e.row...))
	}
	return res
}

type hit struct {
	v   float64
	row []any
}

// measure returns, for one sample, the groups whose measure passes the threshold.
func (r *Rule) measure(s Sample) (map[string]hit, error) {
	col := func(name string) (int, error) {
		if i := slices.Index(s.Cols, name); i >= 0 {
			return i, nil
		}
		return 0, fmt.Errorf("collector %s has no column %q", r.Source, name)
	}
	where := make([]int, len(r.Where))
	group := make([]int, len(r.GroupBy))
	var err error
	for i, w := range r.Where {
		if where[i], err = col(w.Column); err != nil {
			return nil, err
		}
	}
	for i, g := range r.GroupBy {
		if group[i], err = col(g); err != nil {
			return nil, err
		}
	}
	mi, iv, of := -1, -1, -1
	if r.Measure != "count" {
		if mi, err = col(r.Measure); err != nil {
			return nil, err
		}
	}
	if r.PerS {
		if iv, err = col("interval_s"); err != nil {
			return nil, err
		}
	}
	if r.Of != "" {
		if of, err = col(r.Of); err != nil {
			return nil, err
		}
	}

	groups := map[string]hit{}
rows:
	for _, row := range s.Rows {
		for i, w := range r.Where {
			if !textOps[w.Op](text(row[where[i]]), w.Value) {
				continue rows
			}
		}
		var k strings.Builder
		for _, g := range group {
			k.WriteString(text(row[g]))
			k.WriteByte(0)
		}
		key := k.String()
		if r.Measure == "count" {
			h, ok := groups[key]
			if !ok {
				h.row = row
			}
			h.v++
			groups[key] = h
			continue
		}
		v, ok := Number(row[mi])
		if !ok {
			continue
		}
		if iv >= 0 {
			d, ok := Number(row[iv])
			if !ok || d <= 0 {
				continue
			}
			v /= d
		}
		if of >= 0 {
			d, ok := Number(row[of])
			if !ok || d <= 0 {
				continue
			}
			v = 100 * v / d
		}
		if h, ok := groups[key]; !ok || v > h.v {
			groups[key] = hit{v, row}
		}
	}
	limit, pass := r.Params[r.Threshold.Param], numOps[r.Threshold.Op]
	for k, h := range groups {
		if !pass(h.v, limit) {
			delete(groups, k)
		}
	}
	return groups, nil
}

func text(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// Number reads the numeric types the drivers return (and JSON floats); decimals arrive as strings.
func Number(v any) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	case int16:
		return float64(x), true
	case int:
		return float64(x), true
	case uint8:
		return float64(x), true
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	}
	return 0, false
}

// Skips tells whether skip_on excludes platform (the report marks those rules "not applicable").
func (r *Rule) Skips(platform string) bool { return slices.Contains(r.SkipOn, platform) }
