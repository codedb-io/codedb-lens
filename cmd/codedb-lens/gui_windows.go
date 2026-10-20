//go:build windows

package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/codedb-monitorsql/codedb-lens/internal/adapter"
	"github.com/codedb-monitorsql/codedb-lens/internal/engine"
	"github.com/codedb-monitorsql/codedb-lens/internal/i18n"
	"github.com/codedb-monitorsql/codedb-lens/internal/profile"
	"github.com/codedb-monitorsql/codedb-lens/internal/report"
	"github.com/codedb-monitorsql/codedb-lens/internal/saas"
	"github.com/codedb-monitorsql/codedb-lens/internal/update"
	"github.com/codedb-monitorsql/codedb-lens/rules"
)

//go:embed frontend
var frontend embed.FS

// App is bound to the frontend: every exported method is callable as window.go.main.App.<Name>.
type App struct{ ctx context.Context }

type State struct {
	Lang     string            `json:"lang"`
	Version  string            `json:"version"`
	Profiles []profile.Profile `json:"profiles"`
	Last     string            `json:"last"`
	Tokens   string            `json:"tokens"`
	Texts    map[string]string `json:"texts"`
	Landing  string            `json:"landing"`
}

// UpdateNotice returns a localized "new version" message, or "" (also when the check is disabled).
func (a *App) UpdateNotice(lang string) string {
	if update.Disabled(false) {
		return ""
	}
	tag := update.Newer(a.ctx, version)
	if tag == "" {
		return ""
	}
	return fmt.Sprintf(i18n.T(i18n.Normalize(lang), "update_avail"), tag, version, update.Command())
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

func (a *App) Texts(lang string) map[string]string { return i18n.All(i18n.Normalize(lang)) }

// SaveProfile stores settings; an empty password keeps the one already in the credential store.
func (a *App) SaveProfile(p profile.Profile, password string) error {
	f, err := profile.Load()
	if err != nil {
		return err
	}
	f.Put(p)
	if err := f.Save(); err != nil {
		return err
	}
	if password != "" {
		return profile.SetPassword(p.Name, password)
	}
	return nil
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
	lang := i18n.Normalize(firstNonEmpty(p.Lang, i18n.Detect()))
	rep, err := Diagnose(a.ctx, connFor(p, password), lang, "", func(m string) { runtime.EventsEmit(a.ctx, "log", m) },
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
	path, err := rep.Save(firstNonEmpty(p.OutDir, report.DefaultDir(p.Name)))
	if err != nil {
		return RunResult{}, err
	}
	var b strings.Builder
	if err := rep.HTML(&b); err != nil {
		return RunResult{}, err
	}
	return RunResult{HTML: b.String(), Path: path}, nil
}

func (a *App) OpenReport(path string) {
	runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(path))
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
