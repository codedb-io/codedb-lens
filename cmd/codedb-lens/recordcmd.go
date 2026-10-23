package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/license"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/internal/record"
	"github.com/codedb-io/codedb-lens/internal/saas"
	"github.com/codedb-io/codedb-lens/rules"
)

const (
	recordFree = time.Hour      // without a key (Q4)
	recordMax  = 24 * time.Hour // with the free key
)

func recordCmd(args []string) error {
	var o checkOpts
	fs := flag.NewFlagSet("record", flag.ExitOnError)
	connFlags(fs, &o)
	fs.StringVar(&o.out, "out", "", "recording folder (default ./codedb-lens-record-<host>-<date>)")
	dur := fs.Duration("duration", 0, "how long to record: up to 1h without the free key, 24h with it (default: the maximum)")
	_ = fs.Parse(args)

	profiles, err := profile.Load()
	if err != nil {
		return err
	}
	if _, err := resolveConn(&o, profiles); err != nil {
		return err
	}
	l, keyErr := validLicense(o.lang)
	licensed := keyErr == nil
	limit := recordFree
	if licensed {
		limit = recordMax
	}
	switch {
	case *dur == 0:
		*dur = limit
	case *dur < 0:
		return errors.New("--duration must be positive")
	case *dur > recordMax:
		return errors.New(i18n.T(o.lang, "rec_max"))
	case *dur > limit:
		return errors.New(i18n.T(o.lang, "rec_need_key"))
	}

	opts, err := recordOptions(o.conn, o.out, *dur, o.lang, l, licensed)
	if err != nil {
		return err
	}
	var dir string
	lastMinute, wasConnected := -1, true
	opts.Progress = func(s record.Status) {
		if dir == "" { // first cycle: the folder exists now
			dir, _ = filepath.Abs(s.Dir)
			fmt.Fprintf(os.Stderr, i18n.T(o.lang, "rec_started")+"\n", o.conn.Target(), dir, s.Deadline.Local().Format("2006-01-02 15:04"))
			if !licensed {
				fmt.Fprintln(os.Stderr, i18n.T(o.lang, "rec_need_key"))
			}
		}
		if !s.Connected && wasConnected {
			fmt.Fprintf(os.Stderr, i18n.T(o.lang, "rec_down")+"\n", s.LastError)
		}
		wasConnected = s.Connected
		if m := int(time.Since(s.Started).Minutes()); m != lastMinute {
			lastMinute = m
			fmt.Fprintf(os.Stderr, i18n.T(o.lang, "rec_progress")+"\n", time.Since(s.Started).Round(time.Second),
				s.Deadline.Sub(s.Started).Round(time.Second), s.Samples, s.Gaps, humanBytes(s.Bytes))
		}
	}

	// SIGTERM too: docker stop and systemd end the recording cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	meta, err := record.Run(ctx, opts)
	if meta.StopReason == "" {
		return err // never started: nothing was written
	}
	fmt.Fprintf(os.Stderr, "\n"+i18n.T(o.lang, "rec_stopped")+"\n", i18n.T(o.lang, "rec_stop_"+meta.StopReason), dir)
	fmt.Fprintf(os.Stderr, i18n.T(o.lang, "rec_report_hint")+"\n", dir)
	fmt.Fprintf(os.Stderr, "%s\n\n", fmt.Sprintf(i18n.T(o.lang, "cli_cta"), saas.Link(saas.Landing, "cli", "record")))
	return err
}

// recordOptions is the recording setup shared by CLI and GUI: collectors, lock, and the instance limit of the key.
func recordOptions(c adapter.Conn, out string, dur time.Duration, lang string, l *license.License, licensed bool) (record.Options, error) {
	cs, err := record.LoadCollectors(rules.FS, c.Engine)
	if err != nil {
		return record.Options{}, err
	}
	lock := ""
	if p, err := profile.Path(); err == nil {
		lock = filepath.Join(filepath.Dir(p), "record.lock")
	}
	return record.Options{
		Open: func(ctx context.Context) (record.Session, error) {
			s, err := adapter.Open(ctx, c)
			if err != nil {
				return nil, err
			}
			return s, nil
		},
		Collectors: cs,
		Dir:        out,
		Duration:   dur,
		Licensed:   licensed,
		Version:    version,
		LockFile:   lock,
		Check: func(info engine.ServerInfo) error {
			if !licensed {
				return nil
			}
			var other license.ErrOtherInstance
			if err := license.Bind(l, info.Engine, info.Host, time.Now()); errors.As(err, &other) {
				return fmt.Errorf(i18n.T(lang, "rec_other_instance"), other.Bound.Host, other.Bound.Engine,
					saas.Link(saas.Landing, "cli", "instance_limit"))
			} else if err != nil {
				return err
			}
			return nil
		},
	}, nil
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
