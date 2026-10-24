package mcpserver

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLimit(t *testing.T) {
	for _, c := range []struct {
		asked, prof, want int
		capped            bool
	}{
		{0, 0, 500, false},       // defaults
		{100, 0, 100, false},     // below the ceiling
		{900, 0, 500, true},      // above the default ceiling
		{0, 2000, 2000, false},   // the profile raised it
		{3000, 2000, 2000, true}, // above the profile's
		{0, 50000, 10000, false}, // profile value past the cap
		{-1, 0, 500, false},
	} {
		if n, capped := limit(c.asked, c.prof, defMaxRows, capMaxRows); n != c.want || capped != c.capped {
			t.Errorf("limit(%d, %d) = %d, %v; want %d, %v", c.asked, c.prof, n, capped, c.want, c.capped)
		}
	}
}

func TestCut(t *testing.T) {
	big := strings.Repeat("é", 6000) // 2-byte runes: the cut must not split one
	rows := [][]any{{"short", big, int64(1), nil}}
	if !cut(rows) {
		t.Fatal("10 KB cell not cut")
	}
	s := rows[0][1].(string)
	if len(s) > maxCell+len("…") || !utf8.ValidString(s) || rows[0][0] != "short" {
		t.Fatalf("cut: %d bytes, valid %v", len(s), utf8.ValidString(s))
	}
	if cut([][]any{{"short"}}) {
		t.Fatal("short cell reported as cut")
	}
}
