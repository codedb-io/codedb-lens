package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	key := base64.StdEncoding.EncodeToString(pub)
	sums := []byte("abc123  codedb-lens_0.2.0_windows_amd64.zip\ndef456  codedb-lens_0.2.0_linux_amd64.tar.gz\n")
	sig := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, sums)))

	if got, err := Verify(key, sums, sig, "codedb-lens_0.2.0_linux_amd64.tar.gz"); err != nil || got != "def456" {
		t.Fatalf("valid signature: %q %v", got, err)
	}
	tampered := append([]byte("000000  evil.zip\n"), sums...)
	if _, err := Verify(key, tampered, sig, "evil.zip"); err == nil {
		t.Fatal("tampered checksums accepted")
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := Verify(base64.StdEncoding.EncodeToString(other), sums, sig, "codedb-lens_0.2.0_linux_amd64.tar.gz"); err == nil {
		t.Fatal("signature from another key accepted")
	}
	if _, err := Verify("", sums, sig, "x"); err == nil {
		t.Fatal("empty key accepted")
	}
}

func TestDevBuildsNeverNag(t *testing.T) {
	if Newer(t.Context(), "dev") != "" {
		t.Fatal("dev build reported an update")
	}
}
