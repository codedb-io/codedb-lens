package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"math"

	"go.yaml.in/yaml/v3"
)

type Scoring struct {
	Weights      map[Severity]float64 `yaml:"weights"`
	Grades       []Grade              `yaml:"grades"` // highest first
	PartialBelow float64              `yaml:"partial_below"`
	HintCredit   float64              `yaml:"hint_credit"`
}

type Grade struct {
	Letter string  `yaml:"letter"`
	Min    float64 `yaml:"min"`
}

type Score struct {
	Value    float64 `json:"value"` // 0-100
	Letter   string  `json:"letter"`
	Coverage float64 `json:"coverage"` // 0-1, executed / applicable
	Partial  bool    `json:"partial"`
	Executed int     `json:"executed"`
	Applies  int     `json:"applicable"`
}

// CatalogVersion fingerprints the rules of one engine plus scoring.yaml: any change to a rule's
// text, query or parameters, or to the weights, gives a new version.
func CatalogVersion(fsys fs.FS, engine string) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(fsys, engine, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		h.Write([]byte(p + "\x00"))
		h.Write(b)
		return err
	})
	if err != nil {
		return "", err
	}
	b, err := fs.ReadFile(fsys, "scoring.yaml")
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))[:12], err
}

func LoadScoring(fsys fs.FS) (Scoring, error) {
	var s Scoring
	raw, err := fs.ReadFile(fsys, "scoring.yaml")
	if err != nil {
		return s, err
	}
	return s, yaml.Unmarshal(raw, &s)
}

// Compute: grade = 100 * Σ w·r / Σ w over executed rules; r = 1 ok, hint_credit for hints, 0 confirmed.
func (s Scoring) Compute(results []Result) Score {
	var sum, total float64
	var sc Score
	for _, r := range results {
		if r.Status == StatusNotApplies {
			continue
		}
		sc.Applies++
		if r.Status == StatusNoData {
			continue
		}
		sc.Executed++
		w := s.Weights[r.Rule.Severity]
		total += w
		switch {
		case r.Status == StatusOK:
			sum += w
		case r.Confidence == Hint:
			sum += w * s.HintCredit
		}
	}
	if total > 0 {
		sc.Value = math.Round(100*sum/total*10) / 10
	}
	if sc.Applies > 0 {
		sc.Coverage = float64(sc.Executed) / float64(sc.Applies)
	}
	sc.Partial = sc.Coverage < s.PartialBelow
	sc.Letter = s.Grades[len(s.Grades)-1].Letter
	for _, g := range s.Grades {
		if sc.Value >= g.Min {
			sc.Letter = g.Letter
			break
		}
	}
	if sc.Executed == 0 {
		sc.Letter = "?"
	}
	return sc
}
