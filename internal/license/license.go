// Package license verifies the free key offline (Ed25519, per kid), stores it and enforces the local instance limit.
// The only network call is Register, made when the user asks for a key.
package license

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// Keys are the Ed25519 public keys (base64) that sign licenses, by kid. Rotation adds a kid and keeps the old one
// until its last license expires. Test keys live only in _test.go files, never here.
var Keys = map[string]string{
	"2026-1": "RvRL5DciOjaz+Afg2M/c+S5goY9vhxuwf+G4SbtosP0=",
}

const (
	Endpoint = "https://api.codedb.com.br/lens/register"
	// ConsentVersion names the consent text shown before registering (see consent.go). The backend stores it.
	ConsentVersion = "lens-2026-0"
	FeatureCompare = "compare"
	clockSkew      = 24 * time.Hour
)

var (
	ErrMalformed   = errors.New("license: malformed")
	ErrSignature   = errors.New("license: invalid signature")
	ErrUnknownKid  = errors.New("license: unknown key id")
	ErrVersion     = errors.New("license: unsupported format version")
	ErrExpired     = errors.New("license: expired")
	ErrNotYetValid = errors.New("license: issued in the future (check the computer clock)")
	ErrNone        = errors.New("license: none saved")
	ErrUnavailable = errors.New("registration service unavailable, try again later")
)

type License struct {
	V            int      `json:"v"`
	Kid          string   `json:"kid"`
	Sub          string   `json:"sub"`
	Iat          int64    `json:"iat"`
	Exp          int64    `json:"exp"`
	MaxInstances int      `json:"max_instances"`
	Features     []string `json:"features"`
	Raw          string   `json:"-"`
}

// Parse checks a "PAYLOAD.SIGNATURE" token: both parts unpadded base64url, the Ed25519 signature over the ASCII
// bytes of PAYLOAD. Clock skew of 24h is tolerated on iat only; exp is exact.
func Parse(token string, keys map[string]string, now time.Time) (*License, error) {
	token = strings.TrimSpace(token)
	p, s, ok := strings.Cut(token, ".")
	if !ok || strings.Contains(s, ".") {
		return nil, ErrMalformed
	}
	raw, err1 := base64.RawURLEncoding.Strict().DecodeString(p)
	sig, err2 := base64.RawURLEncoding.Strict().DecodeString(s)
	var l License
	if err1 != nil || err2 != nil || len(sig) != ed25519.SignatureSize || json.Unmarshal(raw, &l) != nil {
		return nil, ErrMalformed
	}
	pub, err := base64.StdEncoding.DecodeString(keys[l.Kid])
	if keys[l.Kid] == "" || err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, ErrUnknownKid
	}
	if !ed25519.Verify(pub, []byte(p), sig) {
		return nil, ErrSignature
	}
	switch {
	case l.V != 1:
		return nil, ErrVersion
	case now.Add(clockSkew).Unix() < l.Iat:
		return nil, ErrNotYetValid
	case now.Unix() >= l.Exp:
		return &l, ErrExpired // returned so callers can say when it expired
	}
	l.Raw = token
	return &l, nil
}

func (l *License) Has(feature string) bool { return slices.Contains(l.Features, feature) }

func (l *License) Expires() time.Time { return time.Unix(l.Exp, 0) }

// --- storage: OS credential store, falling back to a 0600 file where there is none (headless Linux) ---

const keyringService, keyringUser = "codeDB Lens", "license"

func dir() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	name := "codedb-lens"
	if os.PathSeparator == '\\' {
		name = "codeDB Lens"
	}
	return filepath.Join(d, name), nil
}

func filePath(name string) (string, error) {
	d, err := dir()
	return filepath.Join(d, name), err
}

// Save verifies the token before storing it: an invalid license is never saved.
func Save(token string) (*License, error) {
	l, err := Parse(token, Keys, timeNow())
	if err != nil {
		return l, err
	}
	if keyring.Set(keyringService, keyringUser, l.Raw) == nil {
		return l, nil
	}
	p, err := filePath("license.txt")
	if err != nil {
		return l, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return l, err
	}
	return l, os.WriteFile(p, []byte(l.Raw), 0o600)
}

// Load returns the saved license, verified now. ErrNone when there is none; ErrExpired comes with the license.
func Load() (*License, error) {
	tok, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		p, perr := filePath("license.txt")
		b, ferr := os.ReadFile(p)
		if perr != nil || ferr != nil {
			return nil, ErrNone
		}
		tok = string(b)
	}
	return Parse(tok, Keys, timeNow())
}

// Delete removes the saved key. The instance binding stays: removing and re-adding the key must not reset it.
func Delete() {
	_ = keyring.Delete(keyringService, keyringUser)
	if p, err := filePath("license.txt"); err == nil {
		_ = os.Remove(p)
	}
}

// --- local instance limit (max_instances = 1). ponytail: a local, removable file by design: it limits scale, it is not DRM ---

type Binding struct {
	Sub     string    `json:"sub"`
	Engine  string    `json:"engine,omitempty"`
	Host    string    `json:"host,omitempty"`
	BoundAt time.Time `json:"bound_at,omitzero"`
	ResetAt time.Time `json:"reset_at,omitzero"`
}

const resetEvery = 30 * 24 * time.Hour

// ErrOtherInstance carries the instance the license is bound to.
type ErrOtherInstance struct{ Bound Binding }

func (e ErrOtherInstance) Error() string {
	return fmt.Sprintf("this free key is in use for %s (%s); run \"codedb-lens license reset\" to move it", e.Bound.Host, e.Bound.Engine)
}

// ErrResetTooSoon tells when the binding can move again.
type ErrResetTooSoon struct{ Next time.Time }

func (e ErrResetTooSoon) Error() string {
	return "the instance can change once every 30 days; next on " + e.Next.Format("2006-01-02")
}

func loadBinding() (Binding, string, error) {
	var b Binding
	p, err := filePath("license-instance.json")
	if err != nil {
		return b, "", err
	}
	if raw, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(raw, &b)
	}
	return b, p, nil
}

func saveBinding(p string, b Binding) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(b, "", "  ")
	return os.WriteFile(p, raw, 0o600)
}

// Bound returns the instance the license is bound to, if any.
func Bound(l *License) (Binding, bool) {
	b, _, _ := loadBinding()
	return b, b.Sub == l.Sub && b.Host != ""
}

// Bind ties the license to one instance on first use; another instance gets ErrOtherInstance.
// A new sub (another registration) starts unbound.
func Bind(l *License, engine, host string, now time.Time) error {
	b, p, err := loadBinding()
	if err != nil {
		return err
	}
	if b.Sub == l.Sub && b.Host != "" {
		if strings.EqualFold(b.Host, host) && b.Engine == engine {
			return nil
		}
		return ErrOtherInstance{b}
	}
	if b.Sub != l.Sub {
		b = Binding{Sub: l.Sub}
	}
	b.Engine, b.Host, b.BoundAt = engine, host, now
	return saveBinding(p, b)
}

// Reset frees the binding so the next compare binds a new instance; at most once every 30 days.
func Reset(l *License, now time.Time) error {
	b, p, err := loadBinding()
	if err != nil {
		return err
	}
	if b.Sub != l.Sub || b.Host == "" {
		return nil // nothing bound
	}
	if !b.ResetAt.IsZero() && now.Sub(b.ResetAt) < resetEvery {
		return ErrResetTooSoon{b.ResetAt.Add(resetEvery)}
	}
	return saveBinding(p, Binding{Sub: l.Sub, ResetAt: now})
}

// --- registration: the only call to the codeDB backend, made when the user asks for a key ---

// Request holds exactly the fields the backend accepts; it rejects unknown fields, so add one only after the backend has it.
type Request struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	Company        string `json:"company,omitempty"`
	Role           string `json:"role,omitempty"`            // dba, dev, infra, it_manager, consultant_msp, other
	InstancesRange string `json:"instances_range,omitempty"` // 1-5, 6-20, 21-100, 100+
	LensVersion    string `json:"lens_version"`
	Lang           string `json:"lang"` // en or pt-BR
	PrivacyAck     bool   `json:"privacy_ack"`
	MarketingOptIn bool   `json:"marketing_opt_in"`
	ConsentVersion string `json:"consent_version"`
}

var (
	Roles  = []string{"dba", "dev", "infra", "it_manager", "consultant_msp", "other"}
	Ranges = []string{"1-5", "6-20", "21-100", "100+"}
)

// Validate mirrors the backend rules so the user sees mistakes before anything is sent.
func (r Request) Validate() error {
	var bad []string
	if n := len([]rune(strings.TrimSpace(r.Name))); n < 1 || n > 200 {
		bad = append(bad, "name")
	}
	if e := strings.TrimSpace(r.Email); len(e) > 254 || !strings.Contains(e, "@") || strings.ContainsAny(e, " \t") {
		bad = append(bad, "email")
	}
	if len([]rune(r.Company)) > 200 {
		bad = append(bad, "company")
	}
	if r.Role != "" && !slices.Contains(Roles, r.Role) {
		bad = append(bad, "role")
	}
	if r.InstancesRange != "" && !slices.Contains(Ranges, r.InstancesRange) {
		bad = append(bad, "instances_range")
	}
	if !r.PrivacyAck {
		bad = append(bad, "privacy_ack")
	}
	if len(bad) > 0 {
		return &InvalidError{Fields: bad}
	}
	return nil
}

type InvalidError struct{ Fields []string }

func (e *InvalidError) Error() string { return "invalid fields: " + strings.Join(e.Fields, ", ") }

type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string {
	return "too many registrations; try again in " + e.RetryAfter.Round(time.Minute).String()
}

var endpoint = Endpoint // tests point this at httptest

var timeNow = time.Now // tests fix the clock

// Register sends the form and returns the verified license (not yet saved).
func Register(ctx context.Context, r Request) (*License, error) {
	r.ConsentVersion = ConsentVersion
	r.Name, r.Email, r.Company = strings.TrimSpace(r.Name), strings.TrimSpace(r.Email), strings.TrimSpace(r.Company)
	if err := r.Validate(); err != nil {
		return nil, err
	}
	body, _ := json.Marshal(r)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "codeDB Lens/"+r.LensVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	var out struct {
		License string `json:"license"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 64<<10)).Decode(&out)
	switch resp.StatusCode {
	case http.StatusOK:
		return Parse(out.License, Keys, timeNow())
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
		// message is "<text>: field, field"; it names fields only, never values
		_, fields, _ := strings.Cut(out.Message, ":")
		return nil, &InvalidError{Fields: strings.Split(strings.ReplaceAll(strings.TrimSpace(fields), " ", ""), ",")}
	case http.StatusTooManyRequests:
		secs, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		return nil, &RateLimitedError{RetryAfter: time.Duration(max(secs, 60)) * time.Second}
	default:
		return nil, fmt.Errorf("%w (HTTP %d)", ErrUnavailable, resp.StatusCode)
	}
}
