package record

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/rules"
)

// The built-in collectors load, validate (read-only SQL, known modes) and cover what the series rules need.
func TestBuiltinCollectors(t *testing.T) {
	want := map[string][]string{
		"mssql": {"requests", "locks", "waits", "counters", "workers", "querystats"},
		"pg":    {"activity", "locks", "dbstats", "connections", "statements"},
	}
	for eng, names := range want {
		cs, err := LoadCollectors(rules.FS, eng)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, c := range cs {
			got = append(got, c.Name)
		}
		if !slices.Equal(got, names) {
			t.Errorf("%s collectors = %v, want %v", eng, got, names)
		}
	}
}

// Every series rule reads a collector that exists on its engine (the columns are checked against real servers in
// the integration test).
func TestSeriesRulesHaveTheirCollector(t *testing.T) {
	for _, eng := range []string{"mssql", "pg"} {
		cs, _ := LoadCollectors(rules.FS, eng)
		rs, err := engine.LoadSeries(rules.FS, eng)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rs {
			if !slices.ContainsFunc(cs, func(c *Collector) bool { return c.Name == r.Source }) {
				t.Errorf("%s: no collector %q on %s", r.ID, r.Source, eng)
			}
		}
	}
}

func TestCollectorValidation(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, sql string }{
		"delta without cumulative": {"- {name: a, every: fast, mode: delta}", "SELECT 1"},
		"write keyword":            {"- {name: a, every: fast, mode: snapshot}", "DELETE FROM t"},
		"unknown interval":         {"- {name: a, every: hourly, mode: snapshot}", "SELECT 1"},
		"keep_top without order":   {"- {name: a, every: fast, mode: snapshot, keep_top: 5}", "SELECT 1"},
		"text without hash":        {"- {name: a, every: fast, mode: snapshot, text: q}", "SELECT 1"},
	} {
		fsys := fstest.MapFS{
			"x/_collect/collect.yaml": {Data: []byte(tc.yaml)},
			"x/_collect/a.sql":        {Data: []byte(tc.sql)},
		}
		if _, err := LoadCollectors(fsys, "x"); err == nil {
			t.Errorf("%s: want a validation error", name)
		}
	}
	fsys := fstest.MapFS{"x/_collect/collect.yaml": {Data: []byte("- {name: a, every: fast, mode: snapshot}")}}
	if _, err := LoadCollectors(fsys, "x"); err == nil || !strings.Contains(err.Error(), "a.sql") {
		t.Errorf("missing SQL: err = %v", err)
	}
}

func TestDelta(t *testing.T) {
	c := &Collector{Name: "waits", Mode: "delta", Key: []string{"wait_type"}, Cumulative: []string{"ms"}}
	cols := []string{"wait_type", "ms", "note"}
	run := func(at time.Duration, rows ...[]any) Sample {
		t.Helper()
		s, ok, err := c.Process(t0.Add(at), cols, rows)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			return Sample{Cols: nil}
		}
		return s
	}

	if s := run(0, []any{"LCK", int64(100), "x"}, []any{"IO", "50.5", "y"}); s.Cols != nil {
		t.Fatal("first run must only set the baseline")
	}
	s := run(15*time.Second,
		[]any{"LCK", int64(130), "x"}, // +30
		[]any{"IO", "50.5", "y"},      // unchanged: dropped
		[]any{"NEW", int64(999), "z"}) // never seen: baseline only
	if got := fmt.Sprint(s.Cols, s.Rows); got != "[wait_type ms note interval_s] [[LCK 30 x 15]]" {
		t.Errorf("second run = %s", got)
	}
	s = run(30*time.Second,
		[]any{"LCK", int64(10), "x"},   // went down (stats cleared): skipped, new baseline
		[]any{"NEW", int64(1000), "z"}) // IO absent this time
	if got := fmt.Sprint(s.Rows); got != "[[NEW 1 z 15]]" {
		t.Errorf("third run = %s", got)
	}
	s = run(45*time.Second, []any{"LCK", int64(15), "x"}, []any{"IO", "60.5", "y"}) // IO back: delta vs 50.5
	if got := fmt.Sprint(s.Rows); got != "[[LCK 5 x 15] [IO 10 y 15]]" {
		t.Errorf("fourth run = %s", got)
	}
}

func TestSingleRowDelta(t *testing.T) { // counters: no key, one row
	c := &Collector{Name: "counters", Mode: "delta", Cumulative: []string{"batch"}}
	cols := []string{"batch", "ple_s"}
	_, _, _ = c.Process(t0, cols, [][]any{{int64(1000), int64(300)}})
	s, ok, err := c.Process(t0.Add(15*time.Second), cols, [][]any{{int64(1600), int64(290)}})
	if err != nil || !ok || fmt.Sprint(s.Rows) != "[[600 290 15]]" {
		t.Errorf("rows = %v, ok %v, err %v", s.Rows, ok, err)
	}
	s, _, _ = c.Process(t0.Add(30*time.Second), cols, [][]any{{int64(1600), int64(280)}}) // idle: still one row (PLE)
	if fmt.Sprint(s.Rows) != "[[0 280 15]]" {
		t.Errorf("unchanged single row = %v", s.Rows)
	}
}

func TestTextsAndKeepTop(t *testing.T) {
	c := &Collector{Name: "requests", Mode: "snapshot", Hash: "h", Text: "q", KeepTop: 2, OrderBy: "cpu"}
	s, ok, err := c.Process(t0, []string{"h", "cpu", "q"}, [][]any{
		{"0x01", int64(5), "SELECT 5"},
		{"0x02", "70.5", "SELECT 70"},
		{"0x03", int64(9), strings.Repeat("é", 3000)},
		{nil, int64(1), "no hash"},
	})
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if got := fmt.Sprint(s.Cols, s.Rows); got != "[h cpu] [[0x02 70.5] [0x03 9]]" {
		t.Errorf("rows = %s", got)
	}
	if len(s.Texts) != 2 || s.Texts["0x02"] != "SELECT 70" || len(s.Texts["0x03"]) > maxText {
		t.Errorf("texts = %d entries, 0x02 = %q, len(0x03) = %d", len(s.Texts), s.Texts["0x02"], len(s.Texts["0x03"]))
	}
	if _, _, err := c.Process(t0, []string{"h", "cpu"}, nil); err == nil {
		t.Error("missing text column: want an error")
	}
}

func TestTextQuery(t *testing.T) {
	c := &Collector{Name: "querystats", TextSQL: "SELECT 1 WHERE h IN ({{hashes}})"}
	q, err := c.TextQuery([]string{"0x1A2B3C4D5E6F7A8B", "-42"})
	if err != nil || q != "SELECT 1 WHERE h IN (0x1A2B3C4D5E6F7A8B, -42)" {
		t.Errorf("q = %q, err = %v", q, err)
	}
	for _, bad := range []string{"1); DROP TABLE t; --", "0x12 OR 1=1", "md5:abc", ""} {
		if _, err := c.TextQuery([]string{bad}); err == nil {
			t.Errorf("%q: want a refusal", bad)
		}
	}
	got := Texts([]string{"hash", "text"}, [][]any{{"0x01", "SELECT 1"}, {"0x02", nil}})
	if len(got) != 1 || got["0x01"] != "SELECT 1" {
		t.Errorf("Texts = %v", got)
	}
}

func TestDue(t *testing.T) {
	fast, slow := &Collector{Every: "fast"}, &Collector{Every: "slow"}
	var runs int
	for tick := range 41 {
		if !fast.Due(tick) {
			t.Fatal("fast collectors run every tick")
		}
		if slow.Due(tick) {
			runs++
		}
	}
	if runs != 3 { // ticks 0, 20, 40
		t.Errorf("slow runs in 41 ticks = %d, want 3", runs)
	}
}

// The report leaves the recorder's own queries out of the top queries by this marker.
func TestCollectorsCarryTheMarker(t *testing.T) {
	for _, eng := range []string{"mssql", "pg"} {
		cs, _ := LoadCollectors(rules.FS, eng)
		for _, c := range cs {
			for _, q := range c.Queries() {
				if !strings.Contains(q, "SELECT /* codeDB Lens */") {
					t.Errorf("%s/%s: a query lacks the /* codeDB Lens */ marker", eng, c.Name)
				}
			}
		}
	}
}
