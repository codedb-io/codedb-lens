package adapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
	_ "github.com/microsoft/go-mssqldb"
)

type MSSQL struct {
	db     *sql.DB
	conn   *sql.Conn // one session for the whole run: sequential and cheap
	info   engine.ServerInfo
	azure  bool   // Azure SQL Database: one database per connection, no USE
	dbName string // connected database (Azure SQL Database only)
}

// ErrAzureMaster: on Azure SQL Database every DMV is scoped to the connected database, so master says nothing.
var ErrAzureMaster = errors.New("Azure SQL Database: set the initial database to the database you want to diagnose, not master")

func mssqlURL(c Conn) string {
	host, instance, port := c.Server, "", c.Port
	if h, p, ok := strings.Cut(host, ","); ok {
		host = h
		port, _ = strconv.Atoi(strings.TrimSpace(p))
	}
	if h, i, ok := strings.Cut(host, `\`); ok {
		host, instance = h, i
	}
	u := &url.URL{Scheme: "sqlserver", Host: host}
	if port > 0 {
		u.Host = fmt.Sprintf("%s:%d", host, port)
	}
	if instance != "" {
		u.Path = instance
	}
	if c.Auth != "windows" {
		u.User = url.UserPassword(c.User, c.Password)
	}
	q := url.Values{}
	q.Set("app name", AppName)
	q.Set("database", firstNonEmpty(c.Database, "master"))
	q.Set("encrypt", "true")
	if c.TrustServerCert {
		q.Set("TrustServerCertificate", "true")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func OpenMSSQL(ctx context.Context, c Conn) (*MSSQL, error) {
	db, err := sql.Open("sqlserver", mssqlURL(c))
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}
	m := &MSSQL{db: db, conn: conn}
	// Low impact: give up on locks fast, lose any deadlock, never wait on writers.
	if _, err := conn.ExecContext(ctx, `SET LOCK_TIMEOUT 1000; SET DEADLOCK_PRIORITY LOW;
		SET TRANSACTION ISOLATION LEVEL READ UNCOMMITTED;`); err != nil {
		m.Close()
		return nil, err
	}
	if err := m.detect(ctx); err != nil {
		m.Close()
		return nil, err
	}
	return m, nil
}

func (m *MSSQL) detect(ctx context.Context) error {
	var edition int
	m.info.Engine = "mssql"
	err := m.conn.QueryRowContext(ctx, `SELECT
		CAST(SERVERPROPERTY('ProductVersion') AS nvarchar(32)),
		ISNULL(CAST(SERVERPROPERTY('ProductMajorVersion') AS int), 0),
		CAST(SERVERPROPERTY('EngineEdition') AS int),
		@@SERVERNAME`).Scan(&m.info.Version, &m.info.Major, &edition, &m.info.Host)
	if err != nil {
		return err
	}
	switch edition {
	case 5:
		m.info.Platform, m.azure = "azure_sqldb", true
		if err := m.conn.QueryRowContext(ctx, `SELECT DB_NAME()`).Scan(&m.dbName); err != nil {
			return err
		}
		if strings.EqualFold(m.dbName, "master") {
			return ErrAzureMaster
		}
	case 8:
		m.info.Platform = "azure_mi"
	default:
		m.info.Platform = "onprem"
		var rds int
		if m.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sys.databases WHERE name = 'rdsadmin'`).Scan(&rds) == nil && rds > 0 {
			m.info.Platform = "rds"
		}
	}
	var start sql.NullString
	// Needs VIEW SERVER STATE; uptime is a caveat, not a requirement. UTC: the server clock may be in another zone.
	if m.conn.QueryRowContext(ctx, `SELECT CONVERT(varchar(19), DATEADD(second, DATEDIFF(second, GETDATE(), GETUTCDATE()), sqlserver_start_time), 126) + 'Z'
		FROM sys.dm_os_sys_info`).Scan(&start) == nil {
		m.info.StartedAt = start.String
	}
	return nil
}

func (m *MSSQL) Info() engine.ServerInfo { return m.info }

func (m *MSSQL) Query(ctx context.Context, q string) ([]string, [][]any, error) {
	rows, err := m.conn.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	return scanAll(rows)
}

// ReadOnly: SQL Server has no read-only session, so the statement runs in a transaction that is rolled back;
// XACT_ABORT makes any error abort it whole. Reads stay READ UNCOMMITTED, like the check: never blocked by writers.
func (m *MSSQL) ReadOnly(ctx context.Context, q string, max int, timeout time.Duration) ([]string, [][]any, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if _, err := m.conn.ExecContext(ctx, "SET XACT_ABORT ON"); err != nil {
		return nil, nil, false, err
	}
	tx, err := m.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, false, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, false, err
	}
	return scanN(rows, max, cancel)
}

func (m *MSSQL) Close() error {
	if m.conn != nil {
		m.conn.Close()
	}
	return m.db.Close()
}

func scanAll(rows *sql.Rows) ([]string, [][]any, error) {
	cols, out, _, err := scanN(rows, 0, nil)
	return cols, out, err
}

// scanN reads up to max rows (0 = all). On a longer result it calls stop, which cancels the query:
// closing the rows would otherwise read the rest from the server.
func scanN(rows *sql.Rows, max int, stop func()) ([]string, [][]any, bool, error) {
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, false, err
	}
	var out [][]any
	for rows.Next() {
		if max > 0 && len(out) == max {
			stop()
			return cols, out, true, nil
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, false, err
		}
		for i := range vals {
			vals[i] = normalize(vals[i])
		}
		out = append(out, vals)
	}
	return cols, out, false, rows.Err()
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

// Databases: online user databases, split by whether this login can open them (HAS_DBACCESS).
func (m *MSSQL) Databases(ctx context.Context) (open, noAccess []string, err error) {
	if m.azure { // other databases on the logical server need their own connection (and profile)
		return []string{m.dbName}, nil, nil
	}
	rows, err := m.conn.QueryContext(ctx, `SELECT name, HAS_DBACCESS(name) FROM sys.databases
		WHERE database_id > 4 AND state_desc = 'ONLINE' AND source_database_id IS NULL ORDER BY name`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var access sql.NullInt64
		if err := rows.Scan(&name, &access); err != nil {
			return nil, nil, err
		}
		if access.Int64 == 1 {
			open = append(open, name)
		} else {
			noAccess = append(noAccess, name)
		}
	}
	return open, noAccess, rows.Err()
}

// QueryDB switches the session to db (USE is a context switch, not a write), runs q, and switches back.
func (m *MSSQL) QueryDB(ctx context.Context, db, q string) ([]string, [][]any, error) {
	if m.azure {
		return m.Query(ctx, q) // Databases returned only the connected one
	}
	if _, err := m.conn.ExecContext(ctx, "USE "+quoteName(db)); err != nil {
		return nil, nil, err
	}
	defer m.conn.ExecContext(context.Background(), "USE master")
	return m.Query(ctx, q)
}

func quoteName(s string) string { return "[" + strings.ReplaceAll(s, "]", "]]") + "]" }
