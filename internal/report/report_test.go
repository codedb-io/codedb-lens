package report

import (
	"testing"
	"time"
)

func TestLocaltime(t *testing.T) {
	f := funcs["localtime"].(func(string) string)
	want := time.Date(2026, 10, 22, 15, 29, 6, 0, time.UTC).Local().Format("2006-01-02 15:04:05")
	if got := f("2026-10-22T15:29:06Z"); got != want {
		t.Errorf("UTC start: got %q, want %q", got, want)
	}
	if got := f("2026-10-22 15:29:06"); got != "2026-10-22 15:29:06" { // reports before v0.5
		t.Errorf("legacy start: got %q", got)
	}
}
