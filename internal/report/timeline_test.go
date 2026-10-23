package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/record"
)

var rec0 = time.Date(2026, 11, 2, 14, 0, 0, 0, time.UTC)

// fakeRecording writes 10 min of PostgreSQL samples: a 45 s lock wait at 14:02, a lost connection from 14:05 to
// 14:06, and query statistics every 5 min.
func fakeRecording(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	meta := record.Meta{LensVersion: "0.8.0", StartedAt: rec0, Deadline: rec0.Add(time.Hour), StoppedAt: rec0.Add(10 * time.Minute),
		StopReason: "user", Intervals: map[string]string{"fast": "15s", "slow": "5m0s"},
		Server: engine.ServerInfo{Engine: "pg", Version: "17.2", Platform: "onprem", Host: "db01.acme.local"}}
	if err := record.WriteMeta(dir, meta); err != nil {
		t.Fatal(err)
	}
	w := record.NewWriter(dir)
	lockCols := []string{"blocked_pid", "blocked_by", "database", "waiting_s", "wait_event", "blocker_pid", "blocker_state",
		"blocker_user", "blocker_application", "blocker_transaction_s", "query_hash"}
	for i := 0; i < 40; i++ {
		at := rec0.Add(time.Duration(i) * 15 * time.Second)
		if at.After(rec0.Add(5*time.Minute)) && at.Before(rec0.Add(6*time.Minute)) {
			_ = w.Write(record.Line{T: at, K: record.KindGap, Err: "connection refused"})
			continue
		}
		var locks [][]any
		if wait := at.Sub(rec0.Add(2 * time.Minute)).Seconds(); wait >= 0 && wait <= 45 {
			locks = [][]any{{int64(200), "{100}", "erp", wait + 1, "relation", int64(100), "idle in transaction", "app_user", "erp-api", wait + 3, "111"}}
			_ = w.WriteText(at, "111", "UPDATE orders SET status = 'paid' WHERE id = 42")
		}
		_ = w.Write(record.Line{T: at, K: "locks", C: lockCols, R: locks})
		_ = w.Write(record.Line{T: at, K: "activity", C: []string{"pid", "state", "wait_event_type", "running_s", "query_hash"},
			R: [][]any{{int64(200), "active", "Lock", 1.0, "111"}}})
		_ = w.Write(record.Line{T: at, K: "connections", C: []string{"connections", "active", "idle_in_transaction", "max_connections"},
			R: [][]any{{int64(20 + i), int64(2), int64(1), int64(97)}}})
		if i == 20 {
			_ = w.WriteText(at, "222", "SELECT * FROM orders WHERE customer_id = $1")
			_ = w.Write(record.Line{T: at, K: "statements", C: []string{"query_hash", "calls", "total_ms", "rows", "blks_read", "blks_hit", "temp_blks", "interval_s"},
				R: [][]any{{"222", 900.0, 4500.5, 900.0, 10.0, 5000.0, 0.0, 300.0}, {"111", 3.0, 45000.0, 3.0, 0.0, 30.0, 0.0, 300.0}}})
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuildTimeline(t *testing.T) {
	tl, err := BuildTimeline(fakeRecording(t), time.Time{}, time.Time{}, "test", "en")
	if err != nil {
		t.Fatal(err)
	}
	if tl.Expected != 40 || tl.Cycles != 37 || tl.Coverage != 0.925 {
		t.Errorf("coverage = %d/%d (%.2f), want 37/40 (3 cycles lost)", tl.Cycles, tl.Expected, tl.Coverage)
	}
	if len(tl.Gaps) != 1 || tl.Gaps[0].From.Sub(rec0) != 5*time.Minute {
		t.Errorf("gaps = %v, want one starting at 14:05", tl.Gaps)
	}

	byID := map[string]engine.Result{}
	for _, r := range tl.Results {
		byID[r.Rule.ID] = r
	}
	blk := byID["PG-REC-BLOCKING-001"]
	if blk.Status != engine.StatusFinding || len(blk.Rows) != 1 {
		t.Fatalf("PG-REC-BLOCKING-001: %s %v (%s)", blk.Status, blk.Rows, blk.Reason)
	}
	if last := blk.Columns[len(blk.Columns)-1]; last != "query" || !strings.Contains(blk.Rows[0][len(blk.Rows[0])-1].(string), "UPDATE orders") {
		t.Errorf("blocking evidence lacks the query text: %v", blk.Rows[0])
	}
	if tl.Results[0].Status != engine.StatusFinding {
		t.Error("findings must come first")
	}
	if r := byID["PG-REC-CONN-001"]; r.Status != engine.StatusOK {
		t.Errorf("connections at most 59/97: %s", r.Status)
	}

	var conn, blocked Chart
	for _, c := range tl.Charts {
		switch c.Key {
		case "tl_ch_conn":
			conn = c
		case "tl_ch_blocked":
			blocked = c
		}
	}
	if len(conn.Points) != 37 || len(blocked.Episodes) != 1 {
		t.Errorf("charts: %d connection points, %d blocking episodes", len(conn.Points), len(blocked.Episodes))
	}
	if len(tl.Top.Rows) != 2 || tl.Top.Rows[0][1] != "111" || tl.Top.Rows[1][0] != "SELECT * FROM orders WHERE customer_id = $1" {
		t.Errorf("top queries = %v", tl.Top.Rows) // ordered by total_ms
	}
}

func TestTimelineHTMLAndRedact(t *testing.T) {
	tl, err := BuildTimeline(fakeRecording(t), time.Time{}, time.Time{}, "test", "pt-BR")
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := tl.HTML(&b); err != nil {
		t.Fatal(err)
	}
	page := b.String()
	for _, want := range []string{"<svg", "<polyline", `class="gap"`, `class="ep"`, "popovertarget=", "Tempo com dados",
		"PG-REC-BLOCKING-001", "UPDATE orders", "db01.acme.local"} {
		if !strings.Contains(page, want) {
			t.Errorf("HTML lacks %q", want)
		}
	}
	if strings.Count(page, "popovertarget=") != strings.Count(page, " popover ") {
		t.Error("every info button needs its popover")
	}

	tl.Redact([]byte("salt"))
	b.Reset()
	_ = tl.HTML(&b)
	page = b.String()
	for _, leak := range []string{"db01.acme.local", "UPDATE orders", "SELECT * FROM orders", "erp-api", "app_user"} {
		if strings.Contains(page, leak) {
			t.Errorf("redacted HTML still shows %q", leak)
		}
	}
}

func TestTimelinePeriod(t *testing.T) {
	dir := fakeRecording(t)
	tl, err := BuildTimeline(dir, rec0.Add(3*time.Minute), rec0.Add(4*time.Minute), "test", "en")
	if err != nil {
		t.Fatal(err)
	}
	if tl.Expected != 4 || tl.Coverage != 1 {
		t.Errorf("14:03-14:04: %d/%d cycles", tl.Cycles, tl.Expected)
	}
	for _, r := range tl.Results {
		if r.Rule.ID == "PG-REC-BLOCKING-001" && r.Status != engine.StatusOK {
			t.Errorf("the lock wait ended at 14:02:45: %s", r.Status)
		}
	}
	path, err := tl.Save(t.TempDir(), true)
	if err != nil || !strings.HasSuffix(path, ".html") || !strings.Contains(path, "report-") {
		t.Errorf("Save = %s, %v", path, err)
	}
}
