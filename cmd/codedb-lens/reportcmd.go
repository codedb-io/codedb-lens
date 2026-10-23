package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/internal/record"
	"github.com/codedb-io/codedb-lens/internal/report"
)

func reportCmd(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	from := fs.String("from", "", `start of the period: "15:04", "2006-01-02 15:04" or RFC 3339 (default: start of the recording)`)
	to := fs.String("to", "", "end of the period, same formats (default: end of the recording)")
	out := fs.String("out", "", "folder for the report (default: the recording folder)")
	lang := fs.String("lang", "", "en or pt-BR (default: system language)")
	redact := fs.Bool("redact", false, "mask host, database, login and application names and drop query text; writes report*.redacted.html/.json")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: codedb-lens report <recording folder> [--from T] [--to T] [--redact] [--out DIR] [--lang en|pt-BR]")
		fs.PrintDefaults()
	}
	// flags may come after the folder
	var dir string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		dir, args = args[0], args[1:]
	}
	_ = fs.Parse(args)
	if dir == "" && fs.NArg() == 1 {
		dir = fs.Arg(0)
	}
	if dir == "" {
		fs.Usage()
		os.Exit(exitError)
	}
	*lang = i18n.Normalize(firstNonEmpty(*lang, i18n.Detect()))

	meta, err := record.ReadMeta(dir)
	if err != nil {
		return err
	}
	f, err := parseWhen(*from, meta.StartedAt)
	if err != nil {
		return fmt.Errorf("--from: %w", err)
	}
	t, err := parseWhen(*to, meta.StartedAt)
	if err != nil {
		return fmt.Errorf("--to: %w", err)
	}
	path, err := saveTimeline(dir, f, t, *lang, *out, *redact)
	if err != nil {
		return err
	}
	fmt.Printf("\n  %s %s\n\n", i18n.T(*lang, "report_saved"), path)
	return nil
}

// saveTimeline builds and writes the report of a recording; shared by CLI and GUI.
func saveTimeline(dir string, from, to time.Time, lang, out string, redact bool) (string, error) {
	tl, err := report.BuildTimeline(dir, from, to, version, lang)
	if err != nil {
		return "", err
	}
	if redact {
		salt, err := profile.RedactSalt()
		if err != nil {
			return "", err
		}
		tl.Redact(salt)
	}
	return tl.Save(firstNonEmpty(out, dir), !from.IsZero() || !to.IsZero())
}

// parseWhen reads a time of the recording. A bare "15:04" is the first such time at or after the recording started,
// in this computer's zone, so a night recording's "02:00" lands on the next day.
func parseWhen(s string, start time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local); err == nil {
		return t, nil
	}
	hm, err := time.ParseInLocation("15:04", s, time.Local)
	if err != nil {
		return time.Time{}, errors.New(`use "15:04", "2006-01-02 15:04" or RFC 3339`)
	}
	ls := start.Local()
	t := time.Date(ls.Year(), ls.Month(), ls.Day(), hm.Hour(), hm.Minute(), 0, 0, time.Local)
	if t.Before(ls.Truncate(time.Minute)) {
		t = t.AddDate(0, 0, 1)
	}
	return t, nil
}
