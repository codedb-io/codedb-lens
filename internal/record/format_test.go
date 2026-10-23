package record

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 2, 13, 59, 30, 0, time.UTC)

func newRecording(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := WriteMeta(dir, Meta{LensVersion: "test", StartedAt: t0}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readKinds(t *testing.T, dir string, from, to time.Time) []string {
	t.Helper()
	var ks []string
	if err := ReadAll(dir, from, to, func(l Line) error { ks = append(ks, l.K); return nil }); err != nil {
		t.Fatal(err)
	}
	return ks
}

func TestRoundTripAndHourRotation(t *testing.T) {
	dir := newRecording(t)
	w := NewWriter(dir)
	steps := []func() error{
		func() error { return w.Write(Line{T: t0, K: "waits", C: []string{"n"}, R: [][]any{{1}}}) },
		func() error { return w.WriteText(t0, "0xAB", "SELECT 1") },
		func() error { return w.WriteText(t0.Add(time.Second), "0xAB", "SELECT 1") }, // same segment: skipped
		func() error { return w.Write(Line{T: t0.Add(15 * time.Second), K: "requests"}) },
		func() error { return w.Write(Line{T: t0.Add(30 * time.Second), K: "waits"}) },  // 14:00:00, new segment
		func() error { return w.WriteText(t0.Add(30*time.Second), "0xAB", "SELECT 1") }, // written again
		func() error { return w.Write(Line{T: t0.Add(time.Minute), K: KindGap, Err: "connection refused"}) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if w.Bytes() == 0 {
		t.Error("Bytes() = 0 after writing")
	}
	segs, _ := filepath.Glob(filepath.Join(dir, "*"+segmentExt))
	if len(segs) != 2 {
		t.Fatalf("segments = %v, want 2 (13h and 14h)", segs)
	}
	got := strings.Join(readKinds(t, dir, time.Time{}, time.Time{}), ",")
	if want := "waits,text,requests,waits,text,gap"; got != want {
		t.Errorf("kinds = %s, want %s", got, want)
	}
	m, err := ReadMeta(dir)
	if err != nil || m.Format != FormatVersion || m.LensVersion != "test" {
		t.Errorf("meta = %+v, %v", m, err)
	}
}

func TestFromTo(t *testing.T) {
	dir := newRecording(t)
	w := NewWriter(dir)
	for i := range 6 { // 13:59:30 ... 14:00:45
		_ = w.Write(Line{T: t0.Add(time.Duration(i) * 15 * time.Second), K: "s"})
	}
	_ = w.Close()
	if n := len(readKinds(t, dir, t0.Add(30*time.Second), t0.Add(60*time.Second))); n != 3 {
		t.Errorf("lines in [14:00:00, 14:00:30] = %d, want 3", n)
	}
	if n := len(readKinds(t, dir, t0.Add(time.Hour), time.Time{})); n != 0 {
		t.Errorf("lines after the recording = %d, want 0", n)
	}
}

// A recording still running, or killed, has an open gzip stream: what was flushed must read, the tail is skipped.
func TestTruncatedLastSegment(t *testing.T) {
	dir := newRecording(t)
	w := NewWriter(dir)
	for i := range 3 {
		_ = w.Write(Line{T: t0.Add(time.Duration(i) * time.Second), K: "s"})
	}
	_ = w.Flush()
	flushed := w.Bytes()
	for i := range 3 {
		_ = w.Write(Line{T: t0.Add(time.Duration(10+i) * time.Second), K: "s", Q: strings.Repeat("x", 500)})
	}
	_ = w.Flush()

	if n := len(readKinds(t, dir, time.Time{}, time.Time{})); n != 6 {
		t.Errorf("open segment: %d lines, want 6", n)
	}

	// Cut the file a few bytes after the first flush, as a crash mid-write would.
	seg := filepath.Join(dir, t0.Format(hourLayout)+segmentExt)
	raw, err := os.ReadFile(seg)
	if err != nil {
		t.Fatal(err)
	}
	cut := newRecording(t)
	if err := os.WriteFile(filepath.Join(cut, filepath.Base(seg)), raw[:flushed+20], 0o600); err != nil {
		t.Fatal(err)
	}
	if n := len(readKinds(t, cut, time.Time{}, time.Time{})); n < 3 {
		t.Errorf("cut segment: %d lines, want at least the 3 flushed", n)
	}
	_ = w.Close()

	// The same damage in a segment that is not the last one is an error, not silence.
	if err := os.WriteFile(filepath.Join(cut, t0.Add(time.Hour).Format(hourLayout)+segmentExt), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReadAll(cut, time.Time{}, time.Time{}, func(Line) error { return nil }); err == nil {
		t.Error("damaged middle segment: want an error")
	}
}

func TestNewerFormatRefused(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, metaFile), []byte(`{"format": 2}`), 0o600)
	if err := ReadAll(dir, time.Time{}, time.Time{}, func(Line) error { return nil }); err == nil || !strings.Contains(err.Error(), "codedb-lens update") {
		t.Errorf("err = %v, want a refusal that says how to update", err)
	}
	if _, err := ReadMeta(t.TempDir()); err == nil {
		t.Error("folder without meta.json: want an error")
	}
}

func TestTruncateKeepsUTF8(t *testing.T) {
	s := truncate(strings.Repeat("é", 3000), maxText) // 2 bytes each
	if len(s) > maxText || !strings.HasPrefix(strings.Repeat("é", 3000), s) || len(s)%2 != 0 {
		t.Errorf("truncate: len %d", len(s))
	}
}
