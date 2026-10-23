//go:build windows

package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/license"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/internal/report"
	"github.com/codedb-io/codedb-lens/internal/saas"
	"github.com/codedb-io/codedb-lens/internal/update"
	"github.com/codedb-io/codedb-lens/rules"
)

//go:embed frontend
var frontend embed.FS

// App is bound to the frontend: every exported method is callable as window.go.main.App.<Name>.
type App struct {
	ctx    context.Context
	mu     sync.Mutex
	cancel context.CancelFunc // of the running diagnosis
}

type State struct {
	Lang     string            `json:"lang"`
	Version  string            `json:"version"`
	Profiles []profile.Profile `json:"profiles"`
	Last     string            `json:"last"`
	Tokens   string            `json:"tokens"`
	Texts    map[string]string `json:"texts"`
	Landing  string            `json:"landing"`
}

// UpdateInfo is the "new version" banner. Self: Lens can update itself (no package manager owns the binary).
type UpdateInfo struct {
	Message string `json:"message"`
	Self    bool   `json:"self"`
}

// UpdateNotice returns the localized banner, or an empty message (also when the check is disabled).
func (a *App) UpdateNotice(lang string) UpdateInfo {
	if update.Disabled(false) {
		return UpdateInfo{}
	}
	tag := update.Newer(a.ctx, version)
	if tag == "" {
		return UpdateInfo{}
	}
	lang = i18n.Normalize(lang)
	if update.Managed() {
		return UpdateInfo{Message: fmt.Sprintf(i18n.T(lang, "update_avail"), tag, version, update.Command())}
	}
	return UpdateInfo{Message: fmt.Sprintf(i18n.T(lang, "gui_update_avail"), tag, version), Self: true}
}

// ApplyUpdate installs the latest release (same Ed25519-verified path as `codedb-lens update`) and restarts the app.
func (a *App) ApplyUpdate() error {
	exe, err := os.Executable() // before Apply renames the running binary
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
	defer cancel()
	tag, err := update.Apply(ctx, version)
	if err != nil || tag == "" {
		return err
	}
	if err := exec.Command(exe).Start(); err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

func (a *App) State() (State, error) {
	f, err := profile.Load()
	lang := i18n.Detect()
	if p, ok := f.Get(f.Last); ok && p.Lang != "" {
		lang = p.Lang
	}
	return State{Lang: lang, Version: version, Profiles: f.Profiles, Last: f.Last, Tokens: report.Tokens, Texts: i18n.All(lang),
		Landing: saas.Link(saas.Landing, "gui", "header")}, err
}

type RuleInfo struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Severity   string `json:"severity"`
	Permission string `json:"permission"`
}

// Rules lists what a check on this engine will look at, for the "what will be checked" panel.
func (a *App) Rules(eng, lang string) []RuleInfo {
	rs, _ := engine.LoadCatalog(rules.FS, eng)
	out := make([]RuleInfo, 0, len(rs))
	for _, r := range rs {
		out = append(out, RuleInfo{r.ID, r.Title[i18n.Normalize(lang)], string(r.Severity), r.Permission})
	}
	return out
}

// History returns the latest runs of a profile (newest first) for the sidebar badge and the history card.
func (a *App) History(p profile.Profile) []report.Summary {
	return report.History(firstNonEmpty(p.OutDir, report.DefaultDir(p.Name)), 8)
}

// DeleteRun permanently removes one run (its .html and .json) from the profile's history.
func (a *App) DeleteRun(p profile.Profile, path string) error {
	return report.DeleteRun(firstNonEmpty(p.OutDir, report.DefaultDir(p.Name)), path)
}

func (a *App) Texts(lang string) map[string]string { return i18n.All(i18n.Normalize(lang)) }

// SaveProfile stores settings; an empty password keeps the one already in the credential store.
// old is the name the profile had when loaded ("" for a new one): a different name renames it,
// carrying its password and (default-folder) history along.
func (a *App) SaveProfile(p profile.Profile, password, old, lang string) error {
	f, err := profile.Load()
	if err != nil {
		return err
	}
	if !strings.EqualFold(p.Name, old) {
		if _, taken := f.Get(p.Name); taken {
			return fmt.Errorf(i18n.T(i18n.Normalize(lang), "gui_name_taken"), p.Name)
		}
	}
	if old != "" && old != p.Name {
		if password == "" {
			password, _ = profile.Password(old)
		}
		if p.OutDir == "" {
			_ = os.Rename(report.DefaultDir(old), report.DefaultDir(p.Name)) // ponytail: no history yet = nothing to move
		}
		f.Delete(old)
	}
	f.Put(p)
	if err := f.Save(); err != nil {
		if old != "" && old != p.Name && password != "" {
			_ = profile.SetPassword(old, password) // Delete(old) already dropped it; the file still has old
		}
		return err
	}
	if password != "" {
		return profile.SetPassword(p.Name, password)
	}
	return nil
}

// DuplicateProfile saves a copy named "<name> (2)" (or the next free number), password included.
func (a *App) DuplicateProfile(name string) (profile.Profile, error) {
	f, err := profile.Load()
	if err != nil {
		return profile.Profile{}, err
	}
	p, ok := f.Get(name)
	if !ok {
		return profile.Profile{}, fmt.Errorf("profile %q not found", name)
	}
	for n := 2; ; n++ {
		p.Name = fmt.Sprintf("%s (%d)", name, n)
		if _, taken := f.Get(p.Name); !taken {
			break
		}
	}
	pw, _ := profile.Password(name)
	return p, a.SaveProfile(p, pw, "", "")
}

// Confirm shows a native Yes/No dialog.
func (a *App) Confirm(title, msg string) bool {
	r, _ := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{Type: runtime.QuestionDialog, Title: title, Message: msg})
	return r == "Yes"
}

func (a *App) DeleteProfile(name string) error {
	f, err := profile.Load()
	if err != nil {
		return err
	}
	f.Delete(name)
	return f.Save()
}

func connFor(p profile.Profile, password string) adapter.Conn {
	c := p.Conn
	c.Password = password
	if c.Password == "" {
		c.Password, _ = profile.Password(p.Name)
	}
	return c
}

// TestConnection returns "engine version (platform)" or an error.
func (a *App) TestConnection(p profile.Profile, password string) (string, error) {
	s, err := adapter.Open(a.ctx, connFor(p, password))
	if err != nil {
		return "", err
	}
	defer s.Close()
	i := s.Info()
	return i.Engine + " " + i.Version + " (" + i.Platform + ")", nil
}

type RunResult struct {
	HTML string `json:"html"`
	Path string `json:"path"`
}

// Run diagnoses the profile, saves the report and returns it for display. Progress goes to the "log" event.
func (a *App) Run(p profile.Profile, password string) (RunResult, error) {
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()
	lang := i18n.Normalize(firstNonEmpty(p.Lang, i18n.Detect()))
	rep, err := Diagnose(ctx, connFor(p, password), lang, "", func(m string) { runtime.EventsEmit(a.ctx, "log", m) },
		func(i, total int, r *engine.Rule, res *engine.Result) {
			ev := map[string]any{"i": i, "total": total, "id": r.ID, "title": r.Title[lang], "status": "running"}
			if res != nil {
				ev["status"], ev["confidence"] = res.Status, res.Confidence
			}
			runtime.EventsEmit(a.ctx, "rule", ev)
		})
	if err != nil {
		return RunResult{}, err
	}
	path, err := saveReport(rep, firstNonEmpty(p.OutDir, report.DefaultDir(p.Name)), false)
	if err != nil {
		return RunResult{}, err
	}
	var b strings.Builder
	if err := rep.HTML(&b); err != nil {
		return RunResult{}, err
	}
	return RunResult{HTML: b.String(), Path: path}, nil
}

// Cancel stops the running diagnosis: the query in flight is cancelled and the connection closed.
func (a *App) Cancel() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

// OpenReport opens a saved report in the default browser (Wails' BrowserOpenURL refuses file:// URLs).
func (a *App) OpenReport(path string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
}

// ReadReport returns a saved HTML report, for the in-app viewer.
func (a *App) ReadReport(path string) (string, error) {
	if !strings.EqualFold(filepath.Ext(path), ".html") {
		return "", fmt.Errorf("not a report: %s", path)
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

// KeyInfo is the free key state for the sidebar and the activation screen.
type KeyInfo struct {
	Valid    bool `json:"valid"`
	Expired  bool `json:"expired"`
	CanRenew bool `json:"can_renew"`
	// ConsentRequired: a renewal was refused until the current Privacy Policy is accepted again.
	ConsentRequired bool              `json:"consent_required,omitempty"`
	DaysLeft        int               `json:"days_left"`
	Expires         string            `json:"expires,omitempty"`
	Instance        string            `json:"instance,omitempty"`
	Consent         map[string]string `json:"consent"` // checkbox labels and notice, versioned in internal/license
	Privacy         string            `json:"privacy"`
	Roles           []string          `json:"roles"`
	Ranges          []string          `json:"ranges"`
}

func (a *App) LicenseStatus(lang string) KeyInfo {
	lang = i18n.Normalize(lang)
	c := license.Consent[lang]
	k := KeyInfo{Consent: map[string]string{"privacy_ack": c.PrivacyAck, "marketing": c.Marketing, "notice": c.Notice, "sent": c.Sent},
		Privacy: saas.Privacy, Roles: license.Roles, Ranges: license.Ranges}
	l, err := license.Load()
	k.Expired = errors.Is(err, license.ErrExpired)
	k.CanRenew = l != nil && license.RenewOpen(l, time.Now())
	if err == nil {
		k.Valid, k.DaysLeft, k.Expires = l.Has(license.FeatureCompare), daysLeft(l), l.Expires().Format("2006-01-02")
		if b, ok := license.Bound(l); ok {
			k.Instance = b.Host
		}
	}
	return k
}

// Register sends the form typed by the user (the only call to the codeDB backend) and saves the key.
func (a *App) Register(r license.Request) (KeyInfo, error) {
	r.Lang, r.LensVersion = i18n.Normalize(r.Lang), version
	l, err := license.Register(a.ctx, r)
	if err != nil {
		return KeyInfo{}, registerError(r.Lang, err)
	}
	if _, err := license.Save(l.Raw); err != nil {
		return KeyInfo{}, err
	}
	return a.LicenseStatus(r.Lang), nil
}

// RenewLicense trades the saved key for a new one. ConsentRequired (no error) asks the page to show the privacy checkbox.
func (a *App) RenewLicense(lang string, privacyAck bool) (KeyInfo, error) {
	lang = i18n.Normalize(lang)
	l, err := license.Load()
	if err != nil && !errors.Is(err, license.ErrExpired) {
		return KeyInfo{}, licenseError(lang, l, err)
	}
	n, err := license.Renew(a.ctx, l, version, privacyAck)
	if errors.Is(err, license.ErrConsentRequired) {
		return KeyInfo{ConsentRequired: true}, nil // the page shows the checkbox and calls again
	}
	if err != nil {
		return KeyInfo{}, renewError(lang, err)
	}
	if _, err := license.Save(n.Raw); err != nil {
		return KeyInfo{}, err
	}
	return a.LicenseStatus(lang), nil
}

// ImportLicense activates a key pasted from the welcome e-mail; no network.
func (a *App) ImportLicense(token, lang string) (KeyInfo, error) {
	lang = i18n.Normalize(lang)
	l, err := license.Save(token)
	if err != nil {
		return KeyInfo{}, licenseError(lang, l, err)
	}
	return a.LicenseStatus(lang), nil
}

func (a *App) RemoveLicense() { license.Delete() }

// Compare diffs the two runs (HTML report paths from History) and returns the comparison for the viewer.
func (a *App) Compare(older, newer, lang string) (RunResult, error) {
	js := func(p string) string { return strings.TrimSuffix(p, ".html") + ".json" }
	c, path, err := compareFiles(i18n.Normalize(lang), js(older), js(newer), "")
	if err != nil {
		return RunResult{}, err
	}
	var b strings.Builder
	if err := c.HTML(&b); err != nil {
		return RunResult{}, err
	}
	return RunResult{HTML: b.String(), Path: path}, nil
}

func (a *App) OpenFolder(path string) { _ = exec.Command("explorer", "/select,", path).Start() }

func (a *App) OpenURL(url string) { runtime.BrowserOpenURL(a.ctx, url) }

func runGUI() bool {
	app := &App{}
	err := wails.Run(&options.App{
		Title:            "codeDB Lens",
		Width:            1280,
		Height:           840,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: &options.RGBA{R: 10, G: 14, B: 23, A: 255},
		AssetServer:      &assetserver.Options{Assets: frontend},
		OnStartup:        func(ctx context.Context) { app.ctx = ctx },
		Bind:             []any{app},
	})
	if err != nil {
		attachConsole()
		println("error:", err.Error())
		os.Exit(1)
	}
	return true
}

// attachConsole reconnects stdout/stderr to the parent terminal, since the release .exe is a GUI-subsystem binary.
func attachConsole() {
	const attachParentProcess = ^uint32(0)
	k := syscall.NewLazyDLL("kernel32.dll")
	if r, _, _ := k.NewProc("AttachConsole").Call(uintptr(attachParentProcess)); r == 0 {
		return // no parent console (or already a console binary in dev)
	}
	// Keep handles the parent redirected (pipes, files); only fill in the missing ones.
	redirected := func(f *os.File) bool {
		t, err := syscall.GetFileType(syscall.Handle(f.Fd()))
		return err == nil && t != 0 // FILE_TYPE_UNKNOWN
	}
	for _, f := range []**os.File{&os.Stdout, &os.Stderr} {
		if redirected(*f) {
			continue
		}
		if h, err := syscall.Open("CONOUT$", syscall.O_RDWR, 0); err == nil {
			*f = os.NewFile(uintptr(h), "CONOUT$")
		}
	}
	if !redirected(os.Stdin) {
		if h, err := syscall.Open("CONIN$", syscall.O_RDWR, 0); err == nil {
			os.Stdin = os.NewFile(uintptr(h), "CONIN$")
		}
	}
}

//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --out rsrc
