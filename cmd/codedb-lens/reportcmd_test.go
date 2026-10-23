package main

import (
	"testing"
	"time"
)

func TestParseWhen(t *testing.T) {
	start := time.Date(2026, 11, 2, 22, 30, 0, 0, time.Local) // a night recording
	for in, want := range map[string]time.Time{
		"":                     {},
		"23:15":                time.Date(2026, 11, 2, 23, 15, 0, 0, time.Local),
		"02:00":                time.Date(2026, 11, 3, 2, 0, 0, 0, time.Local), // after midnight: next day
		"22:30":                start,
		"2026-11-03 01:00":     time.Date(2026, 11, 3, 1, 0, 0, 0, time.Local),
		"2026-11-03T04:00:00Z": time.Date(2026, 11, 3, 4, 0, 0, 0, time.UTC),
	} {
		got, err := parseWhen(in, start)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseWhen(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseWhen("yesterday", start); err == nil {
		t.Error("garbage: want an error")
	}
}
