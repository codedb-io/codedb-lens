package adapter

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
	"github.com/jackc/pgx/v5"
)

type PG struct {
	conn  *pgx.Conn
	cfg   *pgx.ConnConfig
	info  engine.ServerInfo
	perDB map[string]*pgx.Conn // statistics are per database: one extra connection each, closed with the session
}

// Target is the server shown in "Connecting to ...": the host inside a DSN, never the DSN itself (it may hold a password).
func (c Conn) Target() string {
	if c.DSN == "" {
		return c.Server
	}
	if cfg, err := pgx.ParseConfig(c.DSN); err == nil {
		return cfg.Host
	}
	return "DSN"
}

func pgConfig(c Conn) (*pgx.ConnConfig, error) {
	dsn := c.DSN
	if dsn == "" {
		u := &url.URL{Scheme: "postgres", Host: c.Server, Path: "/" + firstNonEmpty(c.Database, "postgres")}
		if c.Port > 0 {
			u.Host = c.Server + ":" + strconv.Itoa(c.Port)
		}
		u.User = url.UserPassword(c.User, c.Password)
		if c.SSLMode != "" {
			u.RawQuery = "sslmode=" + url.QueryEscape(c.SSLMode)
		}
		dsn = u.String()
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid PostgreSQL connection settings: %w", err)
	}
	if c.DSN != "" && c.Password != "" && cfg.Password == "" {
		cfg.Password = c.Password
	}
	if c.DSN != "" && c.Database != "" { // the MCP tools open other databases of a DSN connection
		cfg.Database = c.Database
	}
	// Low impact and read-only at the session level, whatever the login could do.
	cfg.RuntimeParams["application_name"] = AppName
	cfg.RuntimeParams["statement_timeout"] = "10000"
	cfg.RuntimeParams["lock_timeout"] = "1000"
	cfg.RuntimeParams["default_transaction_read_only"] = "on"
	return cfg, nil
}

func OpenPG(ctx context.Context, c Conn) (*PG, error) {
	cfg, err := pgConfig(c)
	if err != nil {
		return nil, err
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	p := &PG{conn: conn, cfg: cfg, info: engine.ServerInfo{Engine: "pg", Host: cfg.Host}, perDB: map[string]*pgx.Conn{}}
	var num int
	var start string
	err = conn.QueryRow(ctx, `SELECT current_setting('server_version'), current_setting('server_version_num')::int,
		to_char(pg_postmaster_start_time() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`).Scan(&p.info.Version, &num, &start)
	if err != nil {
		conn.Close(ctx)
		return nil, err
	}
	p.info.Major, p.info.StartedAt = num/10000, start
	p.info.Platform = p.detectPlatform(ctx)
	return p, nil
}

func (p *PG) detectPlatform(ctx context.Context) string {
	var platform string
	// Managed services leave fingerprints in their settings/roles; default to onprem.
	_ = p.conn.QueryRow(ctx, `SELECT CASE
		WHEN EXISTS (SELECT 1 FROM pg_settings WHERE name LIKE 'aurora%') THEN 'aurora'
		WHEN EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'rds_superuser') THEN 'rds'
		WHEN EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'azure_pg_admin') THEN 'azure_flex'
		ELSE 'onprem' END`).Scan(&platform)
	return firstNonEmpty(platform, "onprem")
}

func (p *PG) Info() engine.ServerInfo { return p.info }

func (p *PG) Query(ctx context.Context, q string) ([]string, [][]any, error) {
	return query(ctx, p.conn, q)
}

// Databases: connectable, non-template databases, split by the CONNECT privilege.
func (p *PG) Databases(ctx context.Context) (open, noAccess []string, err error) {
	rows, err := p.conn.Query(ctx, `SELECT datname, has_database_privilege(datname, 'CONNECT')
		FROM pg_database WHERE datallowconn AND NOT datistemplate ORDER BY datname`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var ok bool
		if err := rows.Scan(&name, &ok); err != nil {
			return nil, nil, err
		}
		if ok {
			open = append(open, name)
		} else {
			noAccess = append(noAccess, name)
		}
	}
	return open, noAccess, rows.Err()
}

func (p *PG) QueryDB(ctx context.Context, db, q string) ([]string, [][]any, error) {
	if db == p.cfg.Database {
		return query(ctx, p.conn, q)
	}
	c := p.perDB[db]
	if c == nil {
		cfg := p.cfg.Copy() // same read-only, low-impact runtime params
		cfg.Database = db
		var err error
		if c, err = pgx.ConnectConfig(ctx, cfg); err != nil {
			return nil, nil, err
		}
		p.perDB[db] = c
	}
	return query(ctx, c, q)
}

// ReadOnly: the session is already read-only; BEGIN READ ONLY repeats it, and the extended protocol refuses
// more than one statement. statement_timeout, for this transaction only, is a backstop one second past the
// caller's timeout, so the caller sees its own deadline.
func (p *PG) ReadOnly(ctx context.Context, q string, max int, timeout time.Duration) ([]string, [][]any, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tx, err := p.conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, false, err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "SELECT set_config('statement_timeout', $1, true)", strconv.FormatInt((timeout + time.Second).Milliseconds(), 10)); err != nil {
		return nil, nil, false, err
	}
	return queryN(ctx, tx, q, max, cancel)
}

// Write lifts the session's read-only default for this statement only: a READ WRITE transaction, or, for Direct,
// the session setting itself (the session is closed after the call).
func (p *PG) Write(ctx context.Context, q string, mode WriteMode, timeout time.Duration) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ms := strconv.FormatInt((timeout + time.Second).Milliseconds(), 10)
	if mode == Direct {
		if _, err := p.conn.Exec(ctx, "SELECT set_config('default_transaction_read_only', 'off', false), set_config('statement_timeout', $1, false)", ms); err != nil {
			return 0, err
		}
		tag, err := p.conn.Exec(ctx, q)
		return tag.RowsAffected(), err
	}
	tx, err := p.conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadWrite})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "SELECT set_config('statement_timeout', $1, true)", ms); err != nil {
		return 0, err
	}
	// Extended protocol: one statement only, so no "...; COMMIT" can end the preview's transaction.
	// (pgx's Exec switches to the simple protocol, which takes several, when there are no arguments.)
	res := tx.Conn().PgConn().ExecParams(ctx, q, nil, nil, nil, nil).Read()
	tag, err := res.CommandTag, res.Err
	if err != nil {
		return 0, err
	}
	if mode == Commit {
		return tag.RowsAffected(), tx.Commit(ctx)
	}
	return tag.RowsAffected(), nil
}

func query(ctx context.Context, conn *pgx.Conn, q string) ([]string, [][]any, error) {
	cols, out, _, err := queryN(ctx, conn, q, 0, nil)
	return cols, out, err
}

// queryN reads up to max rows (0 = all); on a longer result stop cancels the query instead of reading the rest.
func queryN(ctx context.Context, conn interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, q string, max int, stop func()) ([]string, [][]any, bool, error) {
	rows, err := conn.Query(ctx, q)
	if err != nil {
		return nil, nil, false, err
	}
	defer rows.Close()
	var cols []string
	for _, f := range rows.FieldDescriptions() {
		cols = append(cols, f.Name)
	}
	var out [][]any
	for rows.Next() {
		if max > 0 && len(out) == max {
			stop()
			return cols, out, true, nil
		}
		vals, err := rows.Values()
		if err != nil {
			return nil, nil, false, err
		}
		for i := range vals {
			vals[i] = normalize(vals[i])
		}
		out = append(out, vals)
	}
	return cols, out, false, rows.Err()
}

func (p *PG) Close() error {
	for _, c := range p.perDB {
		c.Close(context.Background())
	}
	return p.conn.Close(context.Background())
}
