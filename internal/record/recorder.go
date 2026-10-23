package record

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
)

// Session is the part of an adapter session the recorder uses (adapter.Session satisfies it).
type Session interface {
	Info() engine.ServerInfo
	Query(ctx context.Context, sql string) (cols []string, rows [][]any, err error)
	Close() error
}

const (
	DefaultMaxBytes = 500 << 20        // Q13: clean stop at 500 MB
	queryTimeout    = 10 * time.Second // same as the rules
	staleLock       = 2 * time.Minute
)

type Options struct {
	Open       func(context.Context) (Session, error)
	Collectors []*Collector
	Dir        string        // "" = ./codedb-lens-record-<host>-<AAAAMMDD-HHMM>
	Duration   time.Duration // the CLI and GUI enforce 1 h without a key, 24 h with one (Q4)
	MaxBytes   int64         // 0 = DefaultMaxBytes
	Licensed   bool
	Version    string
	LockFile   string                        // "" = no single-recording lock (tests)
	Check      func(engine.ServerInfo) error // after connecting, before anything is written: license binding
	Progress   func(Status)                  // after every cycle

	tick time.Duration // tests: shorter than FastInterval
}

// Status is what the CLI prints and the GUI shows while recording.
type Status struct {
	Dir       string
	Started   time.Time
	Deadline  time.Time
	Samples   int
	Gaps      int
	Bytes     int64
	Connected bool
	LastError string
}

// ErrBusy: another recording holds the lock on this machine (Q15).
type ErrBusy struct{ Dir string }

func (e ErrBusy) Error() string {
	return "another recording is running on this computer (" + e.Dir + "); stop it first"
}

// Run records until the deadline, the disk cap, an authentication failure or ctx is cancelled (Ctrl+C, GUI stop).
// A connection that fails at start is an error and creates nothing; later failures are gaps while it reconnects.
func Run(ctx context.Context, o Options) (Meta, error) {
	tick := o.tick
	if tick == 0 {
		tick = FastInterval
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = DefaultMaxBytes
	}
	start := time.Now()
	if o.LockFile != "" {
		if err := acquire(o.LockFile, o.Dir, start); err != nil {
			return Meta{}, err
		}
		defer release(o.LockFile)
	}

	sess, err := o.Open(ctx)
	if err != nil {
		return Meta{}, err
	}
	defer func() {
		if sess != nil {
			sess.Close()
		}
	}()
	info := sess.Info()
	if o.Check != nil {
		if err := o.Check(info); err != nil {
			return Meta{}, err
		}
	}
	if o.Dir == "" {
		o.Dir = DefaultDir(info.Host, start)
	}
	if err := newDir(o.Dir); err != nil {
		return Meta{}, err
	}
	if o.LockFile != "" {
		_ = heartbeat(o.LockFile, o.Dir, start)
	}
	meta := Meta{LensVersion: o.Version, Server: info, StartedAt: start.UTC(), Deadline: start.Add(o.Duration).UTC(),
		Intervals: map[string]string{"fast": tick.String(), "slow": (tick * SlowEvery).String()}, Licensed: o.Licensed}
	if err := WriteMeta(o.Dir, meta); err != nil {
		return meta, err
	}

	w := NewWriter(o.Dir)
	st := Status{Dir: o.Dir, Started: start, Deadline: meta.Deadline, Connected: true}
	ticker := time.NewTicker(tick) // a slow cycle drops ticks instead of bursting
	defer ticker.Stop()
	deadline := time.NewTimer(time.Until(meta.Deadline))
	defer deadline.Stop()

	retryAt, backoff := 0, 1 // reconnect: wait 1, 2, 4 ticks (15 s, 30 s, 60 s), then every 4
	var stopErr error
	for i := 0; ; i++ {
		now := time.Now()
		if sess == nil && i >= retryAt {
			if s, err := o.Open(ctx); err == nil {
				sess, backoff, st.Connected = s, 1, true
			} else if authFailed(err) {
				meta.StopReason, stopErr = "error", err
				break
			} else {
				st.LastError, retryAt, backoff = err.Error(), i+backoff, min(backoff*2, 4)
			}
		}
		if sess == nil {
			_ = w.Write(Line{T: now, K: KindGap, Err: st.LastError})
			st.Gaps++
		} else if !cycle(ctx, sess, o.Collectors, w, i, now, &st) && ctx.Err() == nil {
			sess.Close() // every collector failed: treat the connection as lost
			sess, st.Connected, retryAt = nil, false, i+1
		}
		if err := w.Flush(); err != nil {
			meta.StopReason, stopErr = "error", err
			break
		}
		st.Bytes = w.Bytes()
		stayAwake()
		if o.LockFile != "" {
			_ = heartbeat(o.LockFile, o.Dir, now)
		}
		if o.Progress != nil {
			o.Progress(st)
		}
		if st.Bytes >= o.MaxBytes {
			meta.StopReason = "disk_cap"
			break
		}
		select {
		case <-ctx.Done():
			meta.StopReason = "user"
		case <-deadline.C:
			meta.StopReason = "deadline"
		case <-ticker.C:
		}
		if meta.StopReason != "" {
			break
		}
	}
	meta.StoppedAt = time.Now().UTC()
	return meta, errors.Join(stopErr, w.Close(), WriteMeta(o.Dir, meta))
}

// cycle runs the collectors due on tick i. It returns false when every one of them failed.
func cycle(ctx context.Context, s Session, cs []*Collector, w *Writer, i int, now time.Time, st *Status) bool {
	platform := s.Info().Platform
	ran, failed := 0, []string{}
	var lastErr error
	for _, c := range cs {
		if !c.Due(i) || c.Skips(platform) {
			continue
		}
		ran++
		smp, ok, err := runCollector(ctx, s, c, now)
		if err == nil && ok {
			err = writeSample(ctx, s, c, w, now, smp)
		}
		if err != nil {
			failed, lastErr = append(failed, c.Name), err
			continue
		}
		if ok {
			st.Samples++
		}
	}
	if len(failed) > 0 {
		st.Gaps++
		st.LastError = lastErr.Error()
		_ = w.Write(Line{T: now, K: KindGap, C: failed, Err: lastErr.Error()})
	}
	return ran == 0 || len(failed) < ran
}

func runCollector(ctx context.Context, s Session, c *Collector, now time.Time) (Sample, bool, error) {
	qctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	cols, rows, err := s.Query(qctx, c.Query(s.Info().Platform))
	if err != nil {
		if errors.Is(qctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("%s: timeout after %s", c.Name, queryTimeout)
		}
		return Sample{}, false, err
	}
	return c.Process(now, cols, rows)
}

// writeSample writes the texts the segment does not have yet, then the sample. A failed text query only costs texts.
func writeSample(ctx context.Context, s Session, c *Collector, w *Writer, now time.Time, smp Sample) error {
	for h, t := range smp.Texts {
		if err := w.WriteText(now, h, t); err != nil {
			return err
		}
	}
	if c.TextSQL != "" && len(smp.Rows) > 0 {
		hi := slices.Index(smp.Cols, c.Hash)
		if hi < 0 {
			return fmt.Errorf("%s: column %q not in the query result", c.Name, c.Hash)
		}
		var need []string
		for _, r := range smp.Rows {
			if h := str(r[hi]); h != "" && w.NeedsText(now, h) {
				need = append(need, h)
			}
		}
		if q, err := c.TextQuery(need); err == nil && len(need) > 0 {
			qctx, cancel := context.WithTimeout(ctx, queryTimeout)
			if cols, rows, err := s.Query(qctx, q); err == nil {
				for h, t := range Texts(cols, rows) {
					if err := w.WriteText(now, h, t); err != nil {
						cancel()
						return err
					}
				}
			}
			cancel()
		}
	}
	return w.Write(Line{T: now, K: c.Name, C: smp.Cols, R: smp.Rows})
}

// ponytail: matched on the message, not on driver error types, to keep this package driver-free.
// SQL Server 18456 / "Login failed"; PostgreSQL 28P01 / 28000.
func authFailed(err error) bool {
	m := err.Error()
	for _, s := range []string{"Login failed", "18456", "28P01", "28000", "password authentication failed"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// DefaultDir is the recording folder in the current directory (Q13).
func DefaultDir(host string, t time.Time) string {
	h := strings.Trim(unsafeName.ReplaceAllString(host, "-"), "-")
	if h == "" {
		h = "server"
	}
	return "codedb-lens-record-" + h + "-" + t.Format("20060102-1504")
}

// newDir creates the folder; an existing one must be empty, so a recording never mixes with another.
func newDir(dir string) error {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s is not empty; choose another --out", dir)
	}
	return os.MkdirAll(dir, 0o700)
}

// --- one recording per computer (Q15). ponytail: a file with a heartbeat, not an OS lock: same code on Windows,
// Linux and Docker volumes; a crashed recorder frees it after staleLock. ---

type lockInfo struct {
	PID       int       `json:"pid"`
	Dir       string    `json:"dir"`
	Heartbeat time.Time `json:"heartbeat"`
}

func acquire(path, dir string, now time.Time) error {
	if raw, err := os.ReadFile(path); err == nil {
		var li lockInfo
		if json.Unmarshal(raw, &li) == nil && now.Sub(li.Heartbeat) < staleLock && li.PID != os.Getpid() {
			return ErrBusy{li.Dir}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return heartbeat(path, dir, now)
}

func heartbeat(path, dir string, now time.Time) error {
	if abs, err := filepath.Abs(dir); err == nil && dir != "" {
		dir = abs
	}
	raw, _ := json.Marshal(lockInfo{PID: os.Getpid(), Dir: dir, Heartbeat: now.UTC()})
	return os.WriteFile(path, raw, 0o600)
}

func release(path string) {
	if raw, err := os.ReadFile(path); err == nil {
		var li lockInfo
		if json.Unmarshal(raw, &li) == nil && li.PID == os.Getpid() {
			_ = os.Remove(path)
		}
	}
}
