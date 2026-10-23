package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/license"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/internal/report"
	"github.com/codedb-io/codedb-lens/internal/saas"
)

// validLicense returns the saved license if it allows compare, or an error worded for the user.
func validLicense(lang string) (*license.License, error) {
	l, err := license.Load()
	if err == nil && !l.Has(license.FeatureCompare) {
		return nil, fmt.Errorf(i18n.T(lang, "lic_invalid"), "compare")
	}
	return l, licenseError(lang, l, err)
}

// saveReport adds the evolution to the summary and writes the report; the CLI check and the GUI both save through it.
// The evolution needs a valid key, checked offline like compare (read-only: only compare binds the instance).
// redact writes only the masked files. The key is checked against the real host first; the evolution is then computed
// on the masked report, so it chains with earlier masked runs (tokens are stable per machine) and never with plain ones.
func saveReport(rep *report.Report, dir string, redact bool) (string, error) {
	l, err := validLicense(rep.Lang)
	ok := err == nil
	if ok {
		if b, bound := license.Bound(l); bound {
			ok = strings.EqualFold(b.Host, rep.Server.Host) && b.Engine == rep.Server.Engine
		}
	}
	if redact {
		salt, err := profile.RedactSalt()
		if err != nil {
			return "", err
		}
		rep.Redact(salt)
	}
	rep.SetEvolution(dir, ok)
	return rep.Save(dir)
}

// licenseError words license failures for the user.
func licenseError(lang string, l *license.License, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, license.ErrNone):
		return errors.New(i18n.T(lang, "lic_none"))
	case errors.Is(err, license.ErrExpired):
		return fmt.Errorf(i18n.T(lang, "lic_expired"), l.Expires().Format("2006-01-02"))
	}
	return fmt.Errorf(i18n.T(lang, "lic_invalid"), err)
}

// compareFiles is the whole compare, shared by CLI and GUI: license, instance limit, diff, save.
// out "" saves next to the newer report.
func compareFiles(lang, fileA, fileB, out string) (*report.Comparison, string, error) {
	l, err := validLicense(lang)
	if err != nil {
		return nil, "", err
	}
	a, err := report.Load(fileA)
	if err != nil {
		return nil, "", err
	}
	b, err := report.Load(fileB)
	if err != nil {
		return nil, "", err
	}
	if !report.SameInstance(a.Server, b.Server) {
		return nil, "", errors.New(i18n.T(lang, "cmp_different"))
	}
	var other license.ErrOtherInstance
	if err := license.Bind(l, b.Server.Engine, b.Server.Host, time.Now()); errors.As(err, &other) {
		return nil, "", fmt.Errorf(i18n.T(lang, "lic_other_instance"), other.Bound.Host, other.Bound.Engine,
			saas.Link(saas.Landing, "cli", "instance_limit"))
	} else if err != nil {
		return nil, "", err
	}
	c, err := report.Compare(version, lang, a, b, fileA, fileB)
	if err != nil {
		return nil, "", err
	}
	path, err := c.Save(firstNonEmpty(out, filepath.Dir(c.After.File)))
	return c, path, err
}

// latestTwo returns the two newest JSON reports of a profile's folder, older first.
func latestTwo(dir string) (string, string, bool) {
	h := report.History(dir, 2)
	if len(h) < 2 {
		return "", "", false
	}
	js := func(s report.Summary) string { return strings.TrimSuffix(s.Path, ".html") + ".json" }
	return js(h[1]), js(h[0]), true
}

func compareCmd(args []string) error {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	prof := fs.String("profile", "", "compare the two latest runs of this profile (default: last used)")
	out := fs.String("out", "", "folder for the comparison (default: next to the newer report)")
	lang := fs.String("lang", "", "en or pt-BR (default: system language)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: codedb-lens compare [--profile NAME | OLD.json NEW.json] [--out DIR] [--lang en|pt-BR]")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)
	*lang = i18n.Normalize(firstNonEmpty(*lang, i18n.Detect()))

	var a, b string
	switch fs.NArg() {
	case 2:
		a, b = fs.Arg(0), fs.Arg(1)
	case 0:
		profiles, err := profile.Load()
		if err != nil {
			return err
		}
		name := firstNonEmpty(*prof, profiles.Last)
		p, _ := profiles.Get(name)
		var ok bool
		if a, b, ok = latestTwo(firstNonEmpty(p.OutDir, report.DefaultDir(firstNonEmpty(name, "default")))); !ok {
			return errors.New(i18n.T(*lang, "cmp_need_two"))
		}
	default:
		fs.Usage()
		os.Exit(2)
	}
	c, path, err := compareFiles(*lang, a, b, *out)
	if err != nil {
		return err
	}
	printComparison(c, *lang, path)
	return nil
}

func printComparison(c *report.Comparison, lang, path string) {
	x, y := c.Before.Score, c.After.Score
	fmt.Printf("\n  %s: %s → %s (%.0f → %.0f, %+.0f)   %s: %.0f%% → %.0f%%\n", i18n.T(lang, "grade"), x.Letter, y.Letter,
		x.Value, y.Value, c.Delta(), i18n.T(lang, "coverage"), x.Coverage*100, y.Coverage*100)
	fmt.Printf("  %s: %d   %s: %d   %s: %d   %s: %d\n\n", i18n.T(lang, "cmp_new"), len(c.New), i18n.T(lang, "cmp_resolved"),
		len(c.Resolved), i18n.T(lang, "cmp_persisting"), len(c.Persisting), i18n.T(lang, "cmp_changed"), len(c.Changed))
	for _, d := range c.New {
		fmt.Printf("  + [%-8s] %-18s %s\n", d.Rule.Severity, d.Rule.ID, d.Rule.Title[lang])
	}
	for _, d := range c.Resolved {
		fmt.Printf("  - [%-8s] %-18s %s\n", d.Rule.Severity, d.Rule.ID, d.Rule.Title[lang])
	}
	for _, w := range c.Warnings {
		fmt.Printf("\n  ! %s", w.Text(lang))
	}
	fmt.Printf("\n\n  %s %s\n\n", i18n.T(lang, "cmp_saved"), path)
}

func registerCmd(args []string) error {
	var r license.Request
	fs := flag.NewFlagSet("register", flag.ExitOnError)
	fs.StringVar(&r.Name, "name", "", "your name (required)")
	fs.StringVar(&r.Email, "email", "", "your e-mail (required; the key is also sent there)")
	fs.StringVar(&r.Company, "company", "", "company (optional)")
	fs.StringVar(&r.Role, "role", "", "optional: "+strings.Join(license.Roles, ", "))
	fs.StringVar(&r.InstancesRange, "instances", "", "instances you manage, optional: "+strings.Join(license.Ranges, ", "))
	fs.BoolVar(&r.PrivacyAck, "accept-privacy", false, "I have read and agree to the Privacy Policy (required)")
	fs.BoolVar(&r.MarketingOptIn, "marketing", false, "receive news and sales contact from codeDB by e-mail (optional)")
	fs.StringVar(&r.Lang, "lang", "", "en or pt-BR (default: system language)")
	_ = fs.Parse(args)
	lang := i18n.Normalize(firstNonEmpty(r.Lang, i18n.Detect()))
	r.Lang, r.LensVersion = lang, version
	text := license.Consent[lang]
	interactive := term.IsTerminal(int(os.Stdin.Fd()))
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	fmt.Fprintf(os.Stderr, "\n%s\n\n", i18n.T(lang, "reg_intro"))
	if interactive {
		in := bufio.NewReader(os.Stdin)
		ask := func(label string, v *string, opts []string) {
			if *v != "" {
				return
			}
			if opts != nil {
				label += " [" + strings.Join(opts, ", ") + "]"
			}
			for {
				fmt.Fprintf(os.Stderr, "%s: ", label)
				s, _ := in.ReadString('\n')
				*v = strings.TrimSpace(s)
				if opts == nil || *v == "" || slices.Contains(opts, *v) {
					return
				}
			}
		}
		yes := func(label string, v *bool, flagName string) {
			if set[flagName] {
				return
			}
			fmt.Fprintf(os.Stderr, "%s%s", label, i18n.T(lang, "reg_yn"))
			s, _ := in.ReadString('\n')
			s = strings.ToLower(strings.TrimSpace(s))
			*v = strings.HasPrefix(s, "y") || strings.HasPrefix(s, "s")
		}
		ask(i18n.T(lang, "reg_name"), &r.Name, nil)
		ask(i18n.T(lang, "reg_email"), &r.Email, nil)
		ask(i18n.T(lang, "reg_company"), &r.Company, nil)
		ask(i18n.T(lang, "reg_role"), &r.Role, license.Roles)
		ask(i18n.T(lang, "reg_instances"), &r.InstancesRange, license.Ranges)
		fmt.Fprintf(os.Stderr, "\n%s\n", fmt.Sprintf(i18n.T(lang, "reg_privacy_link"), saas.Privacy))
		yes(text.PrivacyAck, &r.PrivacyAck, "accept-privacy")
		yes(text.Marketing, &r.MarketingOptIn, "marketing")
	}
	fmt.Fprintf(os.Stderr, "\n%s\n\n", text.Notice)
	if !r.PrivacyAck {
		return errors.New(i18n.T(lang, "reg_not_sent"))
	}

	l, err := license.Register(context.Background(), r)
	if err != nil {
		return registerError(lang, err)
	}
	if _, err := license.Save(l.Raw); err != nil {
		return err
	}
	fmt.Printf(i18n.T(lang, "lic_saved")+"\n%s\n", l.Expires().Format("2006-01-02"), text.Sent)
	return nil
}

// registerError words registration failures for the user (CLI and GUI).
func registerError(lang string, err error) error {
	var inv *license.InvalidError
	var rl *license.RateLimitedError
	switch {
	case errors.As(err, &inv):
		return fmt.Errorf(i18n.T(lang, "reg_invalid"), strings.Join(inv.Fields, ", "))
	case errors.As(err, &rl):
		return fmt.Errorf(i18n.T(lang, "reg_rate"), waitText(rl.RetryAfter))
	case errors.Is(err, license.ErrUnavailable):
		return errors.New(i18n.T(lang, "reg_unavailable"))
	}
	return err
}

func waitText(d time.Duration) string {
	if d >= time.Hour {
		return fmt.Sprintf("%d h %02d min", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%d min", int(d.Round(time.Minute).Minutes()))
}

func licenseCmd(args []string) error {
	lang := i18n.Normalize(i18n.Detect())
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "import":
		if len(args) != 2 {
			return errors.New("usage: codedb-lens license import <key>")
		}
		l, err := license.Save(args[1])
		if err != nil {
			return licenseError(lang, l, err)
		}
		fmt.Printf(i18n.T(lang, "lic_saved")+"\n", l.Expires().Format("2006-01-02"))
	case "status":
		l, err := validLicense(lang)
		if err != nil {
			return err
		}
		key := "lic_status"
		if daysLeft(l) == 1 {
			key = "lic_status_1"
		}
		fmt.Printf(i18n.T(lang, key)+"\n", l.Expires().Format("2006-01-02"), daysLeft(l),
			strings.Join(l.Features, ", "))
		if b, ok := license.Bound(l); ok {
			fmt.Printf(i18n.T(lang, "lic_bound")+"\n", b.Host, b.Engine)
		} else {
			fmt.Println(i18n.T(lang, "lic_unbound"))
		}
		if license.RenewOpen(l, time.Now()) {
			fmt.Println(i18n.T(lang, "lic_renew_hint"))
		}
	case "reset":
		l, err := validLicense(lang)
		if err != nil {
			return err
		}
		var soon license.ErrResetTooSoon
		if err := license.Reset(l, time.Now()); errors.As(err, &soon) {
			return fmt.Errorf(i18n.T(lang, "lic_reset_soon"), soon.Next.Format("2006-01-02"))
		} else if err != nil {
			return err
		}
		fmt.Println(i18n.T(lang, "lic_reset_ok"))
	case "renew":
		return renewCmd(lang, args[1:])
	case "remove":
		license.Delete()
		fmt.Println(i18n.T(lang, "lic_removed"))
	default:
		return errors.New("usage: codedb-lens license [status|import <key>|renew [--accept-privacy]|reset|remove]")
	}
	return nil
}

// renewCmd trades the saved key (expired is fine) for a new one; asks again for consent only if the backend says so.
func renewCmd(lang string, args []string) error {
	fs := flag.NewFlagSet("license renew", flag.ExitOnError)
	ack := fs.Bool("accept-privacy", false, "accept the current Privacy Policy, if it changed since you registered")
	_ = fs.Parse(args)
	l, err := license.Load()
	if err != nil && !errors.Is(err, license.ErrExpired) {
		return licenseError(lang, l, err)
	}
	fmt.Fprintln(os.Stderr, i18n.T(lang, "lic_renew_sent"))
	n, err := license.Renew(context.Background(), l, version, *ack)
	if errors.Is(err, license.ErrConsentRequired) && !*ack && term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "\n%s\n%s\n%s%s", i18n.T(lang, "lic_renew_consent"),
			fmt.Sprintf(i18n.T(lang, "reg_privacy_link"), saas.Privacy), license.Consent[lang].PrivacyAck, i18n.T(lang, "reg_yn"))
		s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if s = strings.ToLower(strings.TrimSpace(s)); !strings.HasPrefix(s, "y") && !strings.HasPrefix(s, "s") {
			return errors.New(i18n.T(lang, "reg_not_sent"))
		}
		n, err = license.Renew(context.Background(), l, version, true)
	}
	if err != nil {
		return renewError(lang, err)
	}
	if _, err := license.Save(n.Raw); err != nil {
		return err
	}
	fmt.Printf(i18n.T(lang, "lic_renewed")+"\n", n.Expires().Format("2006-01-02"))
	return nil
}

// renewError words renewal failures for the user (CLI and GUI).
func renewError(lang string, err error) error {
	var early *license.TooEarlyError
	switch {
	case errors.As(err, &early):
		return fmt.Errorf(i18n.T(lang, "lic_renew_early"), early.From.Format("2006-01-02"))
	case errors.Is(err, license.ErrRenewGone):
		return errors.New(i18n.T(lang, "lic_renew_gone"))
	case errors.Is(err, license.ErrRenewRejected):
		return errors.New(i18n.T(lang, "lic_renew_rejected"))
	case errors.Is(err, license.ErrConsentRequired):
		return errors.New(i18n.T(lang, "lic_renew_consent"))
	}
	return registerError(lang, err)
}

func daysLeft(l *license.License) int { return int(time.Until(l.Expires()).Hours() / 24) }
