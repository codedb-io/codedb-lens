// Package record samples a server at fixed intervals and keeps the samples in hourly JSONL.gz segments.
package record

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/codedb-io/codedb-lens/internal/engine"
)

// FormatVersion is the recording layout this build writes and the newest it reads. Bump it on any breaking change.
const FormatVersion = 1

const (
	metaFile   = "meta.json"
	segmentExt = ".jsonl.gz"
	hourLayout = "20060102-15"
	maxText    = 4 << 10  // query text kept locally, truncated (Q12)
	maxLine    = 64 << 20 // one sample line; top-N rows with texts stay far below
)

// Meta describes a recording. Written at start, rewritten when the recording stops.
type Meta struct {
	Format      int               `json:"format"`
	LensVersion string            `json:"lens_version"`
	Server      engine.ServerInfo `json:"server"`
	StartedAt   time.Time         `json:"started_at"`
	Deadline    time.Time         `json:"deadline"`
	StoppedAt   time.Time         `json:"stopped_at,omitzero"`
	StopReason  string            `json:"stop_reason,omitempty"` // deadline | user | disk_cap | error
	Intervals   map[string]string `json:"intervals"`
	Licensed    bool              `json:"licensed"`
}

// Line is one entry of a segment: a sample (K = collector), a failed collection (K = "gap") or a query text (K = "text").
type Line struct {
	T   time.Time `json:"t"`
	K   string    `json:"k"`
	C   []string  `json:"c,omitempty"`
	R   [][]any   `json:"r,omitempty"`
	Err string    `json:"err,omitempty"`
	H   string    `json:"h,omitempty"` // text: query hash
	Q   string    `json:"q,omitempty"` // text: query text
}

const (
	KindGap  = "gap"
	KindText = "text"
)

// WriteMeta replaces meta.json atomically, so a reader never sees half a file.
func WriteMeta(dir string, m Meta) error {
	m.Format = FormatVersion
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, metaFile+".tmp")
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, metaFile))
}

// ReadMeta loads meta.json and refuses recordings made by a newer build.
func ReadMeta(dir string) (Meta, error) {
	var m Meta
	raw, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		return m, fmt.Errorf("%s is not a codeDB Lens recording: %w", dir, err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("%s: invalid %s: %w", dir, metaFile, err)
	}
	if m.Format > FormatVersion {
		return m, fmt.Errorf("recording format %d is newer than this codeDB Lens reads (%d); run: codedb-lens update", m.Format, FormatVersion)
	}
	return m, nil
}

// Writer appends lines to the segment of their hour (UTC), opening a new file when the hour changes.
type Writer struct {
	dir    string
	name   string
	f      *os.File
	cw     *countWriter
	gz     *gzip.Writer
	enc    *json.Encoder
	closed int64           // bytes in segments already closed
	seen   map[string]bool // text hashes written to the open segment
}

func NewWriter(dir string) *Writer { return &Writer{dir: dir} }

func (w *Writer) Write(l Line) error {
	l.T = l.T.UTC()
	if err := w.segment(l.T); err != nil {
		return err
	}
	return w.enc.Encode(l)
}

// WriteText writes a query text once per segment, so each hourly file reads on its own.
func (w *Writer) WriteText(t time.Time, hash, text string) error {
	if err := w.segment(t.UTC()); err != nil {
		return err
	}
	if w.seen[hash] {
		return nil
	}
	w.seen[hash] = true
	return w.enc.Encode(Line{T: t.UTC(), K: KindText, H: hash, Q: truncate(text, maxText)})
}

// Flush pushes the open segment to disk; called once per cycle, so a crash loses at most one cycle.
func (w *Writer) Flush() error {
	if w.gz == nil {
		return nil
	}
	return w.gz.Flush()
}

// Bytes is the compressed size written so far (up to the last Flush), for the disk cap.
func (w *Writer) Bytes() int64 {
	if w.cw == nil {
		return w.closed
	}
	return w.closed + w.cw.n
}

func (w *Writer) Close() error {
	if w.gz == nil {
		return nil
	}
	err := errors.Join(w.gz.Close(), w.f.Close())
	w.closed += w.cw.n
	w.f, w.cw, w.gz, w.enc, w.name = nil, nil, nil, nil, ""
	return err
}

func (w *Writer) segment(t time.Time) error {
	name := t.Format(hourLayout) + segmentExt
	if name == w.name {
		return nil
	}
	if err := w.Close(); err != nil {
		return err
	}
	// ponytail: O_APPEND, not O_TRUNC: if the clock jumps back into an hour already written, the file gets a second
	// gzip member, which gzip.Reader reads as one stream. Nothing is overwritten.
	f, err := os.OpenFile(filepath.Join(w.dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	w.f, w.cw, w.name, w.seen = f, &countWriter{w: f}, name, map[string]bool{}
	w.gz = gzip.NewWriter(w.cw)
	w.enc = json.NewEncoder(w.gz)
	return nil
}

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// ReadAll calls fn for every line between from and to (zero = open end), in time order.
// The last segment may end mid-write (crash, or a recording still running): its incomplete tail is skipped.
func ReadAll(dir string, from, to time.Time, fn func(Line) error) error {
	if _, err := ReadMeta(dir); err != nil {
		return err
	}
	segs, err := filepath.Glob(filepath.Join(dir, "*"+segmentExt))
	if err != nil {
		return err
	}
	sort.Strings(segs) // names are AAAAMMDD-HH, so lexical order is time order
	for i, p := range segs {
		hour, err := time.Parse(hourLayout, strings.TrimSuffix(filepath.Base(p), segmentExt))
		if err != nil {
			continue // not ours
		}
		if !from.IsZero() && !hour.Add(time.Hour).After(from) {
			continue
		}
		if !to.IsZero() && hour.After(to) {
			break
		}
		if err := readSegment(p, i == len(segs)-1, from, to, fn); err != nil {
			return err
		}
	}
	return nil
}

func readSegment(path string, last bool, from, to time.Time, fn func(Line) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		if last && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) {
			return nil // created, nothing flushed yet
		}
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 64<<10), maxLine)
	var pending error // a bad line is only fatal if a good one follows it
	for sc.Scan() {
		var l Line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			pending = fmt.Errorf("%s: %w", filepath.Base(path), err)
			continue
		}
		if pending != nil {
			return pending
		}
		if (!from.IsZero() && l.T.Before(from)) || (!to.IsZero() && l.T.After(to)) {
			continue
		}
		if err := fn(l); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil && !(last && errors.Is(err, io.ErrUnexpectedEOF)) {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if pending != nil && !last {
		return pending
	}
	return nil
}
