package profile

import (
	"crypto/rand"
	"os"
	"path/filepath"
)

// RedactSalt is the per-machine secret behind check --redact tokens: random, created on first use next to
// profiles.json and never sent anywhere, so the same name gets the same token on every run here only.
func RedactSalt() ([]byte, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	file := filepath.Join(filepath.Dir(p), "redact.salt")
	if b, err := os.ReadFile(file); err == nil && len(b) == 32 {
		return b, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return nil, err
	}
	return b, os.WriteFile(file, b, 0o600)
}
