// Package update checks GitHub Releases for a newer version (at most once a day, sending no data)
// and replaces the running binary after verifying an Ed25519-signed checksums.txt.
package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const repo = "codedb-monitorsql/codedb-lens"

// PublicKey verifies checksums.txt.sig. Base64 Ed25519 public key; generate with `go run ./tools/sign -genkey`.
// ponytail: empty until the release key exists; Apply refuses to run without it.
var PublicKey = ""

var api = "https://api.github.com/repos/" + repo + "/releases/latest"

type Release struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func latest(ctx context.Context) (*Release, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub releases: %s", resp.Status)
	}
	var r Release
	return &r, json.NewDecoder(resp.Body).Decode(&r)
}

// Disabled reports whether the user turned the version check off.
func Disabled(flag bool) bool { return flag || os.Getenv("CODEDB_LENS_NO_UPDATE") != "" }

type cache struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest"`
}

func cachePath() string {
	dir, _ := os.UserCacheDir()
	return filepath.Join(dir, "codedb-lens", "update-check.json")
}

// Newer returns the latest tag if it is newer than current, hitting the network at most once per 24h.
func Newer(ctx context.Context, current string) string {
	if !semver.IsValid(v(current)) {
		return "" // dev builds never nag
	}
	var c cache
	if raw, err := os.ReadFile(cachePath()); err == nil {
		_ = json.Unmarshal(raw, &c)
	}
	if time.Since(c.Checked) > 24*time.Hour {
		if r, err := latest(ctx); err == nil {
			c = cache{Checked: time.Now(), Latest: r.Tag}
		} else {
			c.Checked = time.Now() // offline: don't retry on every run
		}
		_ = os.MkdirAll(filepath.Dir(cachePath()), 0o755)
		raw, _ := json.Marshal(c)
		_ = os.WriteFile(cachePath(), raw, 0o644)
	}
	if c.Latest != "" && semver.Compare(v(c.Latest), v(current)) > 0 {
		return c.Latest
	}
	return ""
}

func v(s string) string {
	if !strings.HasPrefix(s, "v") {
		return "v" + s
	}
	return s
}

// Command is what the user should run to update, depending on how Lens was installed.
func Command() string {
	exe, _ := os.Executable()
	p := strings.ToLower(filepath.ToSlash(exe))
	switch {
	case strings.Contains(p, "/chocolatey/"):
		return "choco upgrade codedb-lens"
	case strings.Contains(p, "/winget/"):
		return "winget upgrade codeDB.Lens"
	case strings.HasPrefix(p, "/usr/bin/"):
		return "your package manager (apt / dnf)"
	}
	return "codedb-lens update"
}

// Managed is true when a package manager owns the binary and Apply must not touch it.
func Managed() bool { return Command() != "codedb-lens update" }

func assetName(tag string) string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("codedb-lens_%s_%s_%s%s", strings.TrimPrefix(tag, "v"), runtime.GOOS, runtime.GOARCH, ext)
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 200<<20))
}

// Verify checks the signature over checksums.txt and returns the expected SHA-256 for asset.
func Verify(pub string, checksums, sig []byte, asset string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(pub)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return "", errors.New("no valid update signing key in this build")
	}
	rawSig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil || !ed25519.Verify(key, checksums, rawSig) {
		return "", errors.New("checksums.txt signature is invalid; refusing to update")
	}
	for _, line := range strings.Split(string(checksums), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[1] == asset {
			return f[0], nil
		}
	}
	return "", fmt.Errorf("%s not listed in checksums.txt", asset)
}

// Apply downloads, verifies and installs the latest release over the running binary.
func Apply(ctx context.Context, current string) (string, error) {
	if Managed() {
		return "", fmt.Errorf("installed by a package manager; run: %s", Command())
	}
	r, err := latest(ctx)
	if err != nil {
		return "", err
	}
	if semver.Compare(v(r.Tag), v(current)) <= 0 {
		return "", nil
	}
	urls := map[string]string{}
	for _, a := range r.Assets {
		urls[a.Name] = a.URL
	}
	name := assetName(r.Tag)
	for _, n := range []string{name, "checksums.txt", "checksums.txt.sig"} {
		if urls[n] == "" {
			return "", fmt.Errorf("release %s has no %s", r.Tag, n)
		}
	}
	sums, err := download(ctx, urls["checksums.txt"])
	if err != nil {
		return "", err
	}
	sig, err := download(ctx, urls["checksums.txt.sig"])
	if err != nil {
		return "", err
	}
	want, err := Verify(PublicKey, sums, sig, name)
	if err != nil {
		return "", err
	}
	archive, err := download(ctx, urls[name])
	if err != nil {
		return "", err
	}
	if got := sha256.Sum256(archive); hex.EncodeToString(got[:]) != want {
		return "", errors.New("downloaded file does not match checksums.txt; refusing to update")
	}
	bin, err := extract(archive, name)
	if err != nil {
		return "", err
	}
	return r.Tag, replaceSelf(bin)
}

func extract(archive []byte, name string) ([]byte, error) {
	want := "codedb-lens"
	if runtime.GOOS == "windows" {
		want += ".exe"
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == want {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("%s not found in %s", want, name)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, fmt.Errorf("%s not found in %s", want, name)
		}
		if filepath.Base(h.Name) == want {
			return io.ReadAll(tr)
		}
	}
}

// replaceSelf writes the new binary next to the old one and swaps them. Windows allows renaming a running exe.
func replaceSelf(bin []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	tmp, old := exe+".new", exe+".old"
	if err := os.WriteFile(tmp, bin, 0o755); err != nil {
		return err
	}
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Rename(old, exe) // roll back
		return err
	}
	_ = os.Remove(old) // fails on Windows while running; cleaned up next update
	return nil
}
