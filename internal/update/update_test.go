package update

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
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

// The Windows zip carries the console twin; update writes it next to the .exe, also when it did not exist yet.
func TestExtractAndReplaceConsoleTwin(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("zip layout is Windows only")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{"codedb-lens.exe": "gui", "codedb-lens.com": "console"} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	com, err := extract(buf.Bytes(), "x.zip", "codedb-lens.com")
	if err != nil || string(com) != "console" {
		t.Fatalf("extract: %q %v", com, err)
	}
	path := filepath.Join(t.TempDir(), "codedb-lens.com")
	if err := replace(path, com); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "console" {
		t.Fatalf("got %q", got)
	}
}
