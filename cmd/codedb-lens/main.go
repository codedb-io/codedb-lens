// codedb-lens: free, read-only health check for SQL Server and PostgreSQL.

// SQL Server 2017 self-signed certificates have negative serial numbers, which Go rejects by default.
//
//go:debug x509negativeserial=1
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"time"

	"golang.org/x/term"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/profile"
	"github.com/codedb-io/codedb-lens/internal/report"
	"github.com/codedb-io/codedb-lens/internal/saas"
	"github.com/codedb-io/codedb-lens/internal/update"
	"github.com/codedb-io/codedb-lens/rules"
)

var version = "dev" // set by -ldflags at release

const usage = `codeDB Lens %s — free, read-only health check for SQL Server and PostgreSQL

Usage:
  codedb-lens check   --engine mssql|pg [connection flags] [--out DIR] [--lang en|pt-BR]
                      [--fail-on SEVERITY] [--min-coverage PCT]
  codedb-lens compare [--profile NAME | OLD.json NEW.json]   compare two runs (free key)
  codedb-lens register  get the free key (sends only the name and e-mail you type, once)
  codedb-lens license [status|import <key>|reset|remove]
  codedb-lens update    download, verify (Ed25519) and install the latest release
  codedb-lens version

Run "codedb-lens check -h" for all flags.
Passwords are read from CODEDB_LENS_PASSWORD or prompted; never pass them as arguments.
Exit codes: 0 ok, 1 findings at or above --fail-on, 2 error, 3 coverage below --min-coverage.
`

func main() {
	if len(os.Args) < 2 {
		if runGUI() { // Windows build without args opens the app
			return
		}
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	attachConsole()
	var err error
	code := 0
	switch os.Args[1] {
	case "check":
		code, err = check(os.Args[2:])
	case "compare":
		err = compareCmd(os.Args[2:])
	case "register":
		err = registerCmd(os.Args[2:])
	case "license":
		err = licenseCmd(os.Args[2:])
	case "update":
		err = selfUpdate()
	case "version", "--version", "-v":
		fmt.Println("codedb-lens", version)
	case "help", "-h", "--help":
		fmt.Printf(usage, version)
	default:
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitError)
	}
	os.Exit(code)
}

// Exit codes are a public contract (CI pipelines depend on them): never renumber.
const (
	exitFindings = 1
	exitError    = 2
	exitCoverage = 3
)

var severityRank = map[engine.Severity]int{engine.Low: 1, engine.Medium: 2, engine.High: 3, engine.Critical: 4}

// exitCode applies the CI policy. A finding at or above failOn wins over low coverage:
// a confirmed problem is more actionable than an incomplete run.
func exitCode(r *report.Report, failOn engine.Severity, minCoverage float64) int {
	if min := severityRank[failOn]; min > 0 {
		for _, x := range r.Results {
			if x.Status == engine.StatusFinding && severityRank[x.Rule.Severity] >= min {
				return exitFindings
			}
		}
	}
	if r.Score.Coverage*100 < minCoverage {
		return exitCoverage
	}
	return 0
}

type checkOpts struct {
	conn     adapter.Conn
	dsnEnv   string
	out      string
	lang     string
	rulesDir string
	profile  string
}

func check(args []string) (int, error) {
	var o checkOpts
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	fs.StringVar(&o.conn.Engine, "engine", "", "mssql or pg (required)")
	fs.StringVar(&o.conn.Server, "server", "localhost", `server: host, "host,port" or host\instance`)
	fs.IntVar(&o.conn.Port, "port", 0, "port (default 1433 / 5432)")
	fs.StringVar(&o.conn.Database, "database", "", "initial database (default master / postgres)")
	fs.StringVar(&o.conn.Auth, "auth", "sql", "mssql: windows or sql")
	fs.StringVar(&o.conn.User, "user", "", "login name")
	fs.BoolVar(&o.conn.TrustServerCert, "trust-server-cert", false, "accept a self-signed server certificate")
	fs.StringVar(&o.conn.SSLMode, "sslmode", "", "pg: disable, require, verify-full ...")
	fs.StringVar(&o.dsnEnv, "dsn-env", "", "pg: name of an env var holding the connection string")
	fs.StringVar(&o.out, "out", "", "report folder (default Documents/codeDB Lens/reports/<profile>)")
	fs.StringVar(&o.lang, "lang", "", "en or pt-BR (default: system language)")
	fs.StringVar(&o.rulesDir, "rules-dir", "", "use rules from this folder instead of the built-in catalog")
	fs.StringVar(&o.profile, "profile", "", "use a saved profile (connection flags are ignored)")
	save := fs.String("save", "", "save these connection settings as a profile with this name")
	noUpdate := fs.Bool("no-update-check", false, "skip the daily version check")
	failOn := fs.String("fail-on", "", "exit 1 if a finding has this severity or higher: critical, high, medium, low")
	minCoverage := fs.Float64("min-coverage", 0, "exit 3 if coverage is below this percentage (0-100)")
	_ = fs.Parse(args)
	if *failOn != "" && severityRank[engine.Severity(*failOn)] == 0 {
		return 0, fmt.Errorf("--fail-on must be critical, high, medium or low, not %q", *failOn)
	}

	profiles, err := profile.Load()
	if err != nil {
		return 0, err
	}
	if o.profile != "" {
		p, ok := profiles.Get(o.profile)
		if !ok {
			return 0, fmt.Errorf("profile %q not found", o.profile)
		}
		o.conn, o.lang, o.out = p.Conn, firstNonEmpty(o.lang, p.Lang), firstNonEmpty(o.out, p.OutDir)
		if pw, err := profile.Password(p.Name); err == nil {
			o.conn.Password = pw
		}
	}
	o.lang = i18n.Normalize(firstNonEmpty(o.lang, i18n.Detect()))
	if o.conn.Engine == "" {
		return 0, errors.New("--engine is required (mssql or pg)")
	}
	if o.dsnEnv != "" {
		o.conn.DSN = os.Getenv(o.dsnEnv)
		if o.conn.DSN == "" {
			return 0, fmt.Errorf("environment variable %s is empty", o.dsnEnv)
		}
	}
	if o.conn.Password == "" {
		if err := readPassword(&o); err != nil {
			return 0, err
		}
	}
	if *save != "" {
		o.profile = *save
		profiles.Put(profile.Profile{Name: *save, Conn: o.conn, Lang: o.lang, OutDir: o.out})
		if err := profiles.Save(); err != nil {
			return 0, err
		}
		if o.conn.Password != "" {
			if err := profile.SetPassword(*save, o.conn.Password); err != nil {
				fmt.Fprintln(os.Stderr, "warning: no OS credential store; the password was not saved:", err)
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	newer := make(chan string, 1)
	go func() { // runs alongside the diagnosis; never delays or fails it
		if update.Disabled(*noUpdate) {
			newer <- ""
			return
		}
		uctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		newer <- update.Newer(uctx, version)
	}()
	rep, err := Diagnose(ctx, o.conn, o.lang, o.rulesDir, func(msg string) { fmt.Fprintln(os.Stderr, msg) }, nil)
	if err != nil {
		return 0, err
	}
	path, err := rep.Save(firstNonEmpty(o.out, report.DefaultDir(firstNonEmpty(o.profile, o.conn.Server))))
	if err != nil {
		return 0, err
	}
	printSummary(rep, o.lang, path)
	select {
	case tag := <-newer:
		if tag != "" {
			fmt.Fprintf(os.Stderr, i18n.T(o.lang, "update_avail")+"\n\n", tag, version, update.Command())
		}
	case <-time.After(time.Second):
	}
	return exitCode(rep, engine.Severity(*failOn), *minCoverage), nil
}

func selfUpdate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	tag, err := update.Apply(ctx, version)
	switch {
	case err != nil:
		return err
	case tag == "":
		fmt.Println("codedb-lens", version, "is up to date")
	default:
		fmt.Println("updated to", tag)
	}
	return nil
}

// Diagnose is the whole check: connect, run the catalog, score. Shared by CLI and GUI.
// onRule (optional) sees each rule before (res == nil) and after it runs.
func Diagnose(ctx context.Context, c adapter.Conn, lang, rulesDir string, log func(string), onRule func(i, total int, r *engine.Rule, res *engine.Result)) (*report.Report, error) {
	var catalog fs.FS = rules.FS
	if rulesDir != "" {
		catalog = os.DirFS(rulesDir)
	}
	ruleset, err := engine.LoadCatalog(catalog, c.Engine)
	if err != nil {
		return nil, err
	}
	scoring, err := engine.LoadScoring(catalog)
	if err != nil {
		return nil, err
	}
	catVersion, err := engine.CatalogVersion(catalog, c.Engine)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	log(fmt.Sprintf(i18n.T(lang, "connecting"), c.Server))
	sess, err := adapter.Open(ctx, c)
	if err != nil {
		if ctx.Err() != nil { // stopped while connecting: drivers wrap it as a dial error
			return nil, ctx.Err()
		}
		return nil, err
	}
	defer sess.Close()
	info := sess.Info()
	log(fmt.Sprintf(i18n.T(lang, "connected"), info.Engine, info.Version, info.Platform))
	results := engine.Run(ctx, sess, ruleset, func(i, total int, r *engine.Rule, res *engine.Result) {
		if res == nil {
			log(fmt.Sprintf(i18n.T(lang, "running"), r.ID, i+1, total))
		}
		if onRule != nil {
			onRule(i, total, r, res)
		}
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rep := report.New(version, lang, info, scoring.Compute(results), results)
	rep.CatalogVersion, rep.Duration = catVersion, time.Since(start).Round(time.Second).String()
	return rep, nil
}

func readPassword(o *checkOpts) error {
	if o.conn.Auth == "windows" && o.conn.Engine == "mssql" {
		return nil
	}
	if p, ok := os.LookupEnv("CODEDB_LENS_PASSWORD"); ok {
		o.conn.Password = p
		return nil
	}
	if o.conn.DSN != "" { // password in the DSN, PGPASSWORD or .pgpass
		return nil
	}
	if o.conn.Engine == "pg" && os.Getenv("PGPASSWORD") != "" {
		o.conn.Password = os.Getenv("PGPASSWORD")
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("no password: set CODEDB_LENS_PASSWORD")
	}
	fmt.Fprintf(os.Stderr, i18n.T(o.lang, "password_prompt"), o.conn.User)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	o.conn.Password = string(b)
	return err
}

func printSummary(r *report.Report, lang, path string) {
	s := r.Score
	grade := fmt.Sprintf("%s (%.0f/100)", s.Letter, s.Value)
	if s.Limited {
		grade = s.Letter
	}
	fmt.Printf("\n  %s: %s   %s: %.0f%%", i18n.T(lang, "grade"), grade, i18n.T(lang, "coverage"), s.Coverage*100)
	if s.Limited {
		fmt.Printf("\n  "+i18n.T(lang, "limited_help"), s.Applies, s.Rules, r.Server.Platform)
	} else if s.Partial {
		fmt.Printf("  [%s]", i18n.T(lang, "partial"))
	}
	fmt.Printf("\n  %s: %d   %s: %d   %s: %d\n\n", i18n.T(lang, "findings"), r.Count(engine.StatusFinding),
		i18n.T(lang, "no_data"), r.Count(engine.StatusNoData), i18n.T(lang, "passed"), r.Count(engine.StatusOK))
	for _, x := range r.Results {
		if x.Status == engine.StatusFinding {
			fmt.Printf("  [%-8s] %-18s %s\n", x.Rule.Severity, x.Rule.ID, x.Rule.Title[lang])
		}
	}
	fmt.Printf("\n  %s %s\n", i18n.T(lang, "report_saved"), path)
	fmt.Printf("  %s\n\n", fmt.Sprintf(i18n.T(lang, "cli_cta"), saas.Link(saas.Landing, "cli", "summary")))
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
