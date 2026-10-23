package record

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
)

// fakeDB answers every collector with one row; down makes every query (and new connections) fail.
type fakeDB struct {
	opens   atomic.Int32
	down    atomic.Bool
	authBad atomic.Bool
}

type fakeSession struct{ db *fakeDB }

func (s fakeSession) Info() engine.ServerInfo {
	return engine.ServerInfo{Engine: "pg", Host: "db01:5432", Platform: "onprem"}
}
func (s fakeSession) Close() error { return nil }
func (s fakeSession) Query(ctx context.Context, q string) ([]string, [][]any, error) {
	if s.db.down.Load() {
		return nil, nil, errors.New("connection reset by peer")
	}
	return []string{"n"}, [][]any{{int64(1)}}, nil
}

func (db *fakeDB) open(ctx context.Context) (Session, error) {
	db.opens.Add(1)
	switch {
	case db.authBad.Load():
		return nil, errors.New(`FATAL: password authentication failed for user "x" (SQLSTATE 28P01)`)
	case db.down.Load():
		return nil, errors.New("connection refused")
	}
	return fakeSession{db}, nil
}

func testOptions(t *testing.T, db *fakeDB) Options {
	return Options{
		Open:       db.open,
		Collectors: []*Collector{{Name: "activity", Every: "fast", Mode: "snapshot", SQL: "S"}},
		Dir:        filepath.Join(t.TempDir(), "rec"),
		Duration:   time.Hour,
		tick:       5 * time.Millisecond,
	}
}

func lines(t *testing.T, dir string) map[string]int {
	t.Helper()
	n := map[string]int{}
	if err := ReadAll(dir, time.Time{}, time.Time{}, func(l Line) error { n[l.K]++; return nil }); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRecordUntilDeadline(t *testing.T) {
	o := testOptions(t, &fakeDB{})
	o.Duration = 100 * time.Millisecond
	meta, err := Run(context.Background(), o)
	if err != nil || meta.StopReason != "deadline" {
		t.Fatalf("stop = %q, err = %v", meta.StopReason, err)
	}
	if got, _ := ReadMeta(o.Dir); got.StopReason != "deadline" || got.StoppedAt.IsZero() || got.Server.Host != "db01:5432" {
		t.Errorf("meta on disk = %+v", got)
	}
	if n := lines(t, o.Dir)["activity"]; n < 5 {
		t.Errorf("%d samples in 100 ms of 5 ms ticks", n)
	}
}

func TestRecordStopsOnCancel(t *testing.T) {
	o := testOptions(t, &fakeDB{})
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	o.Progress = func(s Status) {
		if s.Samples >= 3 {
			once.Do(cancel)
		}
	}
	meta, err := Run(ctx, o)
	if err != nil || meta.StopReason != "user" {
		t.Fatalf("stop = %q, err = %v", meta.StopReason, err)
	}
}

// The connection drops mid-recording: gaps are written while it retries with backoff, then sampling resumes.
func TestRecordReconnects(t *testing.T) {
	db := &fakeDB{}
	o := testOptions(t, db)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cycles := 0
	o.Progress = func(s Status) {
		cycles++
		switch cycles {
		case 3:
			db.down.Store(true)
		case 15:
			db.down.Store(false)
		case 30:
			cancel()
		}
	}
	if _, err := Run(ctx, o); err != nil {
		t.Fatal(err)
	}
	n := lines(t, o.Dir)
	if n[KindGap] < 5 || n["activity"] < 10 {
		t.Errorf("lines = %v, want gaps while down and samples after", n)
	}
	if opens := db.opens.Load(); opens < 3 || opens > 8 { // backoff: not one attempt per tick
		t.Errorf("%d connection attempts", opens)
	}
}

func TestRecordStopsOnAuthFailure(t *testing.T) {
	db := &fakeDB{}
	o := testOptions(t, db)
	cycles := 0
	o.Progress = func(Status) {
		if cycles++; cycles == 2 {
			db.down.Store(true) // password changed while recording
			db.authBad.Store(true)
		}
	}
	meta, err := Run(context.Background(), o)
	if meta.StopReason != "error" || err == nil || !strings.Contains(err.Error(), "28P01") {
		t.Errorf("stop = %q, err = %v", meta.StopReason, err)
	}
	if got, _ := ReadMeta(o.Dir); got.StopReason != "error" {
		t.Errorf("meta on disk = %q", got.StopReason)
	}
}

func TestRecordDiskCap(t *testing.T) {
	o := testOptions(t, &fakeDB{})
	o.MaxBytes = 1
	meta, err := Run(context.Background(), o)
	if err != nil || meta.StopReason != "disk_cap" {
		t.Errorf("stop = %q, err = %v", meta.StopReason, err)
	}
}

func TestRecordFailsBeforeWriting(t *testing.T) {
	db := &fakeDB{}
	db.down.Store(true)
	o := testOptions(t, db)
	if _, err := Run(context.Background(), o); err == nil {
		t.Error("unreachable server at start: want an error")
	}
	o.Check = func(engine.ServerInfo) error { return errors.New("key bound to another instance") }
	db.down.Store(false)
	if _, err := Run(context.Background(), o); err == nil {
		t.Error("Check refused: want an error")
	}
	if _, err := os.Stat(o.Dir); !os.IsNotExist(err) {
		t.Error("nothing may be written when the recording never started")
	}
	os.MkdirAll(o.Dir, 0o700)
	os.WriteFile(filepath.Join(o.Dir, "x"), nil, 0o600)
	o.Check = nil
	if _, err := Run(context.Background(), o); err == nil {
		t.Error("non-empty folder: want an error")
	}
}

func TestSingleRecordingLock(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "record.lock")
	now := time.Now()
	if err := os.WriteFile(lock, []byte(`{"pid": -1, "dir": "/elsewhere", "heartbeat": "`+now.UTC().Format(time.RFC3339)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var busy ErrBusy
	if err := acquire(lock, "mine", now); !errors.As(err, &busy) || busy.Dir != "/elsewhere" {
		t.Errorf("live lock of another process: err = %v", err)
	}
	if err := acquire(lock, "mine", now.Add(staleLock+time.Second)); err != nil {
		t.Errorf("stale lock must be taken over: %v", err)
	}
	release(lock)
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Error("release must remove our own lock")
	}

	o := testOptions(t, &fakeDB{})
	o.LockFile, o.Duration = lock, 30*time.Millisecond
	if _, err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Error("lock left behind after the recording")
	}
}

func TestDefaultDir(t *testing.T) {
	got := DefaultDir(`SRV01\SQL2022,1433`, time.Date(2026, 11, 2, 9, 5, 0, 0, time.Local))
	if got != "codedb-lens-record-SRV01-SQL2022-1433-20261102-0905" {
		t.Errorf("DefaultDir = %s", got)
	}
}
