package license

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// Test key and sample licenses from the codeDB backend (kid test-1). The key is accepted only here.
var testKeys = map[string]string{"test-1": "Vu7Y8Eb09gJLopsFSTtAJZwDxYuYDGUJ6hMuU+S583M="}

const (
	sampleValid      = "eyJ2IjoxLCJraWQiOiJ0ZXN0LTEiLCJzdWIiOiJhM2ViYmUxOS03MThkLTQ0ZDEtYWQyZS00MmZjMzI3ZDQyYmEiLCJpYXQiOjE3OTI2MDA4NzYsImV4cCI6MTgwMDM3Njg3NiwibWF4X2luc3RhbmNlcyI6MSwiZmVhdHVyZXMiOlsiY29tcGFyZSJdfQ.DDV6Rx9gpJGYv1xZil9CwiSIQDcXr0Ps1SI_8jcQIgZPY1HJCBsqV_ulYX4iUSVOgNIdp7HPHU5ouipt5lWdDw"
	sampleExpired    = "eyJ2IjoxLCJraWQiOiJ0ZXN0LTEiLCJzdWIiOiJhM2ViYmUxOS03MThkLTQ0ZDEtYWQyZS00MmZjMzI3ZDQyYmEiLCJpYXQiOjE3ODM5NjA4NzYsImV4cCI6MTc5MTczNjg3NiwibWF4X2luc3RhbmNlcyI6MSwiZmVhdHVyZXMiOlsiY29tcGFyZSJdfQ.a0_torRCqoqwSd_Q8JmnGxHucRX71XmlX5y2KtX9a8oCX96oWpYZUlsyeSEfLfa01KKV4QcsSvPjSRxcp7GICA"
	sampleUnknownKid = "eyJ2IjoxLCJraWQiOiJ1bmtub3duLTkiLCJzdWIiOiJhM2ViYmUxOS03MThkLTQ0ZDEtYWQyZS00MmZjMzI3ZDQyYmEiLCJpYXQiOjE3OTI2MDA4NzYsImV4cCI6MTgwMDM3Njg3NiwibWF4X2luc3RhbmNlcyI6MSwiZmVhdHVyZXMiOlsiY29tcGFyZSJdfQ.55kLieuOrD3xmc-omAMoqgD8WZuBsNzIV9l-zTcFNELcfXe66jlDDIwDzuhXZWI7grYdAIWc_PTMB-k9gwHBCQ"
)

var now = time.Date(2026, 10, 21, 18, 0, 0, 0, time.UTC) // fixed: sampleValid expires 2027-01-19

// sign makes a license with a fresh key pair, for cases the samples don't cover.
func sign(t *testing.T, payload map[string]any) (string, map[string]string) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	raw, _ := json.Marshal(payload)
	p := base64.RawURLEncoding.EncodeToString(raw)
	return p + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(p))),
		map[string]string{payload["kid"].(string): base64.StdEncoding.EncodeToString(pub)}
}

func payload(mod func(map[string]any)) map[string]any {
	p := map[string]any{"v": 1, "kid": "k", "sub": "s", "iat": now.Unix(), "exp": now.Add(90 * 24 * time.Hour).Unix(),
		"max_instances": 1, "features": []string{"compare"}}
	if mod != nil {
		mod(p)
	}
	return p
}

func TestParseSamples(t *testing.T) {
	l, err := Parse(sampleValid, testKeys, now)
	if err != nil || !l.Has(FeatureCompare) || l.Sub != "a3ebbe19-718d-44d1-ad2e-42fc327d42ba" ||
		l.Expires().UTC().Format("2006-01-02") != "2027-01-19" || l.Raw != sampleValid {
		t.Fatalf("valid: %+v %v", l, err)
	}
	if _, err := Parse(sampleExpired, testKeys, now); !errors.Is(err, ErrExpired) {
		t.Errorf("expired: %v", err)
	}
	if _, err := Parse(sampleUnknownKid, testKeys, now); !errors.Is(err, ErrUnknownKid) {
		t.Errorf("unknown kid: %v", err)
	}
	// the release key set must reject the test key
	if _, ok := Keys["test-1"]; ok {
		t.Error("test-1 must not be a built-in key")
	}
	if _, err := Parse(sampleValid, Keys, now); !errors.Is(err, ErrUnknownKid) {
		t.Errorf("test license with built-in keys: %v", err)
	}
}

func TestParseRejects(t *testing.T) {
	p, s, _ := strings.Cut(sampleValid, ".")
	flip := p[:20] + string(p[20]^1) + p[21:]
	for name, tok := range map[string]string{
		"tampered":   flip + "." + s,
		"no dot":     p + s,
		"three dots": sampleValid + ".x",
		"padding":    p + "=." + s,
		"std b64":    strings.ReplaceAll(sampleValid, "_", "/"),
		"empty":      "",
	} {
		if _, err := Parse(tok, testKeys, now); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	tok, keys := sign(t, payload(func(m map[string]any) { m["v"] = 2 }))
	if _, err := Parse(tok, keys, now); !errors.Is(err, ErrVersion) {
		t.Errorf("v2: %v", err)
	}
	tok, keys = sign(t, payload(func(m map[string]any) { m["iat"] = now.Add(23 * time.Hour).Unix() }))
	if _, err := Parse(tok, keys, now); err != nil {
		t.Errorf("iat within skew: %v", err)
	}
	tok, keys = sign(t, payload(func(m map[string]any) { m["iat"] = now.Add(25 * time.Hour).Unix() }))
	if _, err := Parse(tok, keys, now); !errors.Is(err, ErrNotYetValid) {
		t.Errorf("iat beyond skew: %v", err)
	}
	tok, keys = sign(t, payload(func(m map[string]any) { m["exp"] = now.Unix() }))
	if _, err := Parse(tok, keys, now); !errors.Is(err, ErrExpired) {
		t.Errorf("exp is exact: %v", err)
	}
	tok, keys = sign(t, payload(func(m map[string]any) { m["features"] = []string{"record"} }))
	if l, err := Parse(tok, keys, now); err != nil || l.Has(FeatureCompare) {
		t.Errorf("features without compare: %v", err)
	}
}

func isolate(t *testing.T) {
	d := t.TempDir()
	t.Setenv("AppData", d)
	t.Setenv("XDG_CONFIG_HOME", d)
	t.Setenv("HOME", d)
	keyring.MockInit()
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = time.Now })
}

func TestBindAndReset(t *testing.T) {
	isolate(t)
	l := &License{Sub: "s1"}
	if err := Bind(l, "mssql", "SRV1", now); err != nil {
		t.Fatal(err)
	}
	if err := Bind(l, "mssql", "srv1", now); err != nil {
		t.Errorf("same instance: %v", err)
	}
	var other ErrOtherInstance
	if err := Bind(l, "mssql", "SRV2", now); !errors.As(err, &other) || other.Bound.Host != "SRV1" {
		t.Errorf("other instance: %v", err)
	}
	if err := Reset(l, now); err != nil {
		t.Fatalf("first reset: %v", err)
	}
	if err := Bind(l, "mssql", "SRV2", now); err != nil {
		t.Errorf("bind after reset: %v", err)
	}
	if err := Reset(l, now.Add(24*time.Hour)); !errors.As(err, new(ErrResetTooSoon)) {
		t.Errorf("second reset within 30 days: %v", err)
	}
	if err := Reset(l, now.Add(31*24*time.Hour)); err != nil {
		t.Errorf("reset after 30 days: %v", err)
	}
	if err := Bind(&License{Sub: "s2"}, "pg", "SRV3", now); err != nil {
		t.Errorf("new registration starts unbound: %v", err)
	}
}

func TestRegister(t *testing.T) {
	var got map[string]any
	status, retry, lic := 200, "", sampleValid
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = nil
		_ = json.NewDecoder(r.Body).Decode(&got)
		if retry != "" {
			w.Header().Set("Retry-After", retry)
		}
		w.WriteHeader(status)
		switch status {
		case 200:
			_ = json.NewEncoder(w).Encode(map[string]string{"license": lic, "expires_at": "x"})
		case 400:
			_, _ = w.Write([]byte(`{"error":"invalid_request","message":"Campos inválidos: email"}`))
		}
	}))
	defer srv.Close()
	isolate(t)
	api = srv.URL
	defer func() { api = API }()
	Keys["test-1"] = testKeys["test-1"]
	defer delete(Keys, "test-1")

	req := Request{Name: " Maria ", Email: "maria@example.com", LensVersion: "0.5.0", Lang: "pt-BR", PrivacyAck: true}
	if _, err := Register(context.Background(), Request{Name: "x", Email: "x@y"}); err == nil || got != nil {
		t.Fatal("must not send without privacy_ack")
	}
	l, err := Register(context.Background(), req)
	if err != nil || l.Sub == "" {
		t.Fatalf("200: %v", err)
	}
	want := map[string]bool{"name": true, "email": true, "lens_version": true, "lang": true, "privacy_ack": true,
		"marketing_opt_in": true, "consent_version": true}
	for k := range got {
		if !want[k] {
			t.Errorf("unexpected field sent: %s", k)
		}
	}
	if len(got) != len(want) || got["name"] != "Maria" || got["consent_version"] != ConsentVersion || got["marketing_opt_in"] != false {
		t.Errorf("body: %v", got)
	}

	status = 400
	var inv *InvalidError
	if _, err := Register(context.Background(), req); !errors.As(err, &inv) || !strings.Contains(inv.Error(), "email") {
		t.Errorf("400: %v", err)
	}
	status, retry = 429, "3600"
	var rl *RateLimitedError
	if _, err := Register(context.Background(), req); !errors.As(err, &rl) || rl.RetryAfter != time.Hour {
		t.Errorf("429: %v", err)
	}
	status, retry = 503, ""
	if _, err := Register(context.Background(), req); !errors.Is(err, ErrUnavailable) {
		t.Errorf("503: %v", err)
	}
	status, lic = 200, sampleUnknownKid
	if _, err := Register(context.Background(), req); err == nil {
		t.Error("200 with an invalid license must fail")
	}
}

func TestSaveLoad(t *testing.T) {
	isolate(t)
	if _, err := Load(); !errors.Is(err, ErrNone) {
		t.Fatalf("empty: %v", err)
	}
	Keys["test-1"] = testKeys["test-1"]
	defer delete(Keys, "test-1")
	if _, err := Save(sampleExpired); err == nil {
		t.Fatal("saved an expired license")
	}
	if _, err := Save(sampleValid); err != nil {
		t.Fatal(err)
	}
	if l, err := Load(); err != nil || l.Raw != sampleValid {
		t.Fatalf("load: %v", err)
	}
	Delete()
	if _, err := Load(); !errors.Is(err, ErrNone) {
		t.Errorf("after delete: %v", err)
	}
}

func TestRenew(t *testing.T) {
	var got map[string]any
	status, body := 200, ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = nil
		_ = json.NewDecoder(r.Body).Decode(&got)
		if r.URL.Path != "/renew" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	api = srv.URL
	defer func() { api = API }()
	Keys["test-1"] = testKeys["test-1"]
	defer delete(Keys, "test-1")
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()
	old, err := Parse(sampleExpired, testKeys, now)
	if !errors.Is(err, ErrExpired) {
		t.Fatal(err)
	}

	body = `{"license":"` + sampleValid + `","expires_at":"2027-01-19T00:00:00Z"}`
	l, err := Renew(context.Background(), old, "0.5.0", false)
	if err != nil || l.Sub != old.Sub {
		t.Fatalf("200: %v", err)
	}
	if len(got) != 3 || got["license"] != sampleExpired || got["consent_version"] != ConsentVersion {
		t.Errorf("body without privacy_ack: %v", got)
	}
	_, _ = Renew(context.Background(), old, "0.5.0", true)
	if got["privacy_ack"] != true {
		t.Errorf("privacy_ack not sent: %v", got)
	}
	status, body = 409, `{"error":"too_early","renew_from":"2027-01-05T12:00:00Z"}`
	var early *TooEarlyError
	if _, err := Renew(context.Background(), old, "0.5.0", false); !errors.As(err, &early) || early.From.Day() != 5 {
		t.Errorf("409: %v", err)
	}
	for code, want := range map[int]error{400: ErrRenewRejected, 404: ErrRenewGone, 412: ErrConsentRequired, 503: ErrUnavailable} {
		status, body = code, `{"error":"x"}`
		if _, err := Renew(context.Background(), old, "0.5.0", false); !errors.Is(err, want) {
			t.Errorf("%d: %v", code, err)
		}
	}

	exp := old.Expires()
	for d, want := range map[int]bool{-15: false, -13: true, 29: true, 31: false} {
		if RenewOpen(old, exp.Add(time.Duration(d)*24*time.Hour)) != want {
			t.Errorf("RenewOpen at %+d days: want %v", d, want)
		}
	}
}
