package i18n

import "testing"

func TestBothLanguagesHaveEveryKey(t *testing.T) {
	for _, a := range []string{"en", "pt-BR"} {
		for _, b := range []string{"en", "pt-BR"} {
			for k := range messages[a] {
				if _, ok := messages[b][k]; !ok {
					t.Errorf("key %q exists in %s but not in %s", k, a, b)
				}
			}
		}
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{"pt_BR.UTF-8": "pt-BR", "pt-br": "pt-BR", "en-US": "en", "de_DE": "en", "": "en"} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
