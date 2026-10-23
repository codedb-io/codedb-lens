// overhead measures what `codedb-lens record` costs a busy server (Q5: target below 1% of CPU).
//
// It runs a synthetic OLTP load in three phases: A1 and A2 (half the length each) without recording around B with
// `codedb-lens record` running, so drift (table bloat, caches warming up) does not count as overhead. It reports the server CPU spent by the recorder's own queries (every collector query carries
// /* codeDB Lens */), the load's throughput in both phases and the recording size per hour, as a Markdown table.
//
//	CODEDB_LENS_PASSWORD=... go run ./tools/overhead -engine pg -server localhost -port 15417 -user postgres -lens ./codedb-lens -minutes 30
//
// Writes only to the disposable test database it is pointed at (table lens_load). Never point it at a real server.
//
//go:debug x509negativeserial=1
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
)

var (
	eng     = flag.String("engine", "pg", "mssql or pg")
	server  = flag.String("server", "localhost", "host")
	port    = flag.Int("port", 0, "port (default 1433 / 5432)")
	user    = flag.String("user", "", "login (default sa / postgres)")
	lens    = flag.String("lens", "./codedb-lens", "codedb-lens binary")
	minutes = flag.Float64("minutes", 30, "length of each phase")
	workers = flag.Int("workers", 8, "load connections")
	cores   = flag.Int("cores", runtime.NumCPU(), "pg: server CPU cores (SQL Server reports its own)")
)

const rows = 100_000

func main() {
	flag.Parse()
	pw := os.Getenv("CODEDB_LENS_PASSWORD")
	ms := *eng == "mssql"
	if *port == 0 {
		*port = map[bool]int{true: 1433, false: 5432}[ms]
	}
	if *user == "" {
		*user = map[bool]string{true: "sa", false: "postgres"}[ms]
	}
	u := url.URL{User: url.UserPassword(*user, pw), Host: *server + ":" + strconv.Itoa(*port)}
	driver := "pgx"
	if ms {
		driver, u.Scheme, u.RawQuery = "sqlserver", "sqlserver", "TrustServerCertificate=true&app+name=lens-overhead"
	} else {
		u.Scheme, u.Path, u.RawQuery = "postgres", "/postgres", "sslmode=disable&application_name=lens-overhead"
	}
	db, err := sql.Open(driver, u.String())
	check(err)
	db.SetMaxOpenConns(*workers + 2)
	ctx := context.Background()
	setup(ctx, db, ms)

	phase := time.Duration(*minutes * float64(time.Minute))
	fmt.Fprintf(os.Stderr, "phase A1: %s of load without recording\n", phase/2)
	tpsA1 := load(ctx, db, ms, phase/2)

	fmt.Fprintf(os.Stderr, "phase B: %s of load while recording\n", phase)
	if !ms { // pg_stat_statements keeps the first text it saw per query: start clean so every recorder query carries the marker
		_, err = db.ExecContext(ctx, `SELECT pg_stat_statements_reset()`)
		check(err)
	}
	before := lensCPU(ctx, db, ms)
	tmp, err := os.MkdirTemp("", "lens-overhead-")
	check(err)
	defer os.RemoveAll(tmp)
	rec := filepath.Join(tmp, "rec")
	args := []string{"record", "--engine", *eng, "--server", *server, "--port", strconv.Itoa(*port), "--user", *user,
		"--duration", (phase + 30*time.Second).String(), "--out", rec, "--lang", "en"}
	if ms {
		args = append(args, "--trust-server-cert")
	} else {
		args = append(args, "--sslmode", "disable")
	}
	cmd := exec.Command(*lens, args...)
	// own config folder: no lock or key binding shared with the user's real setup
	cmd.Env = append(os.Environ(), "APPDATA="+tmp, "XDG_CONFIG_HOME="+tmp, "HOME="+tmp)
	cmd.Stderr = os.Stderr
	check(cmd.Start())
	time.Sleep(5 * time.Second) // connect and first cycle
	tpsB := load(ctx, db, ms, phase)
	_ = cmd.Process.Signal(os.Interrupt)
	if runtime.GOOS == "windows" { // no SIGINT for child processes on Windows: the recording dir is all we need
		_ = cmd.Process.Kill()
	}
	_ = cmd.Wait()
	cpu := lensCPU(ctx, db, ms) - before
	fmt.Fprintf(os.Stderr, "phase A2: %s of load without recording\n", phase/2)
	tpsA := (tpsA1 + load(ctx, db, ms, phase/2)) / 2

	n := *cores
	if ms {
		check(db.QueryRowContext(ctx, "SELECT cpu_count FROM sys.dm_os_sys_info").Scan(&n))
	}
	size := dirSize(rec)
	what := "CPU time of the recorder's queries (sys.dm_exec_query_stats)"
	if !ms {
		what = "execution time of the recorder's queries (pg_stat_statements, CPU plus I/O wait: an upper bound)"
	}
	fmt.Printf("| | |\n|---|---|\n")
	fmt.Printf("| Engine | %s, %d cores, %d load connections |\n", *eng, n, *workers)
	fmt.Printf("| Phases | %s without, %s recording, %s without |\n", phase/2, phase, phase/2)
	fmt.Printf("| Recorder server time | %.3f s (%s) |\n", cpu.Seconds(), what)
	fmt.Printf("| Recorder share of server CPU | **%.4f%%** (target < 1%%) |\n", 100*cpu.Seconds()/(phase.Seconds()*float64(n)))
	fmt.Printf("| Recorder share of one core | %.3f%% |\n", 100*cpu.Seconds()/phase.Seconds())
	fmt.Printf("| Load throughput | %.0f ops/s without, %.0f ops/s with recording (%+.1f%%) |\n", tpsA, tpsB, 100*(tpsB-tpsA)/tpsA)
	fmt.Printf("| Recording size | %.2f MB per hour |\n", float64(size)/(1<<20)/phase.Hours())
}

func setup(ctx context.Context, db *sql.DB, ms bool) {
	if ms {
		exec := func(q string) { _, err := db.ExecContext(ctx, q); check(err) }
		exec(`IF DB_ID('lens_load') IS NULL CREATE DATABASE lens_load`)
		exec(`IF OBJECT_ID('lens_load.dbo.t') IS NULL EXEC ('USE lens_load; CREATE TABLE dbo.t (id int PRIMARY KEY, v int NOT NULL, pad char(200) NOT NULL);
			INSERT dbo.t SELECT TOP (` + strconv.Itoa(rows) + `) ROW_NUMBER() OVER (ORDER BY (SELECT 1)), 0, ''x'' FROM sys.all_objects a CROSS JOIN sys.all_objects b')`)
		return
	}
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS lens_load (id int PRIMARY KEY, v int NOT NULL, pad char(200) NOT NULL)`)
	check(err)
	_, err = db.ExecContext(ctx, `INSERT INTO lens_load SELECT g, 0, 'x' FROM generate_series(1, `+strconv.Itoa(rows)+`) g ON CONFLICT DO NOTHING`)
	check(err)
	_, err = db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS pg_stat_statements`)
	check(err)
}

// load runs point reads, point updates and a range scan every 50 operations; returns operations per second.
func load(ctx context.Context, db *sql.DB, ms bool, d time.Duration) float64 {
	read, upd, scan := `SELECT v FROM lens_load WHERE id = $1`, `UPDATE lens_load SET v = v + 1 WHERE id = $1`,
		`SELECT sum(v) FROM lens_load WHERE id BETWEEN $1 AND $1 + 5000`
	if ms {
		read, upd, scan = `SELECT v FROM lens_load.dbo.t WHERE id = @p1`, `UPDATE lens_load.dbo.t SET v = v + 1 WHERE id = @p1`,
			`SELECT SUM(v) FROM lens_load.dbo.t WHERE id BETWEEN @p1 AND @p1 + 5000`
	}
	var ops atomic.Int64
	end := time.Now().Add(d)
	var wg sync.WaitGroup
	for range *workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; time.Now().Before(end); i++ {
				id := rand.IntN(rows-5000) + 1
				q := read
				switch {
				case i%50 == 0:
					q = scan
				case i%3 == 0:
					q = upd
				}
				if _, err := db.ExecContext(ctx, q, id); err == nil {
					ops.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	return float64(ops.Load()) / d.Seconds()
}

// lensCPU is the cumulative server time of the recorder's queries. The probe's own text is excluded.
func lensCPU(ctx context.Context, db *sql.DB, ms bool) time.Duration {
	var us float64
	if ms {
		check(db.QueryRowContext(ctx, `SELECT /* overhead-probe */ COALESCE(SUM(qs.total_worker_time), 0)
			FROM sys.dm_exec_query_stats qs CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) t
			WHERE t.text LIKE '%/* codeDB Lens */%' AND t.text NOT LIKE '%overhead-probe%'`).Scan(&us))
		return time.Duration(us) * time.Microsecond
	}
	var ms2 float64
	check(db.QueryRowContext(ctx, `SELECT COALESCE(sum(total_exec_time), 0) FROM pg_stat_statements
		WHERE query LIKE '%/* codeDB Lens */%'`).Scan(&ms2))
	return time.Duration(ms2 * float64(time.Millisecond))
}

func dirSize(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if i, err := d.Info(); err == nil {
				n += i.Size()
			}
		}
		return nil
	})
	return n
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
