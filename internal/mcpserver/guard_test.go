package mcpserver

import (
	"encoding/json"
	"os"
	"testing"
)

// The attack corpus (plan §7.3), shared with the integration tests, which run the refused cases against a login
// that can write.
type guardCorpus struct {
	Allowed, Refused map[string][]string // "both", "mssql", "pg"
}

func TestGuard(t *testing.T) {
	raw, err := os.ReadFile("testdata/guard_corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var c guardCorpus
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	for _, eng := range []string{"mssql", "pg"} {
		n := 0
		for _, group := range []string{"both", eng} {
			for _, q := range c.Allowed[group] {
				if w, ok := guard(eng, q); !ok {
					t.Errorf("%s: refused %q (%s)", eng, q, w)
				}
			}
			for _, q := range c.Refused[group] {
				if _, ok := guard(eng, q); ok {
					t.Errorf("%s: allowed %q", eng, q)
				}
			}
			n += len(c.Allowed[group]) + len(c.Refused[group])
		}
		if n < 60 {
			t.Errorf("%s: only %d cases", eng, n)
		}
	}
	if w, _ := guard("pg", "SELECT 1 FROM t FOR UPDATE"); w != "UPDATE" {
		t.Errorf("refusal names the word: got %q", w)
	}
	if _, ok := guard("oracle", "SELECT 1"); ok {
		t.Error("unknown engine allowed")
	}
}
