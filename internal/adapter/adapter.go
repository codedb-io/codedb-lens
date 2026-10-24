// Package adapter opens read-only, low-impact sessions on SQL Server and PostgreSQL.
package adapter

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/codedb-io/codedb-lens/internal/engine"
)

const AppName = "codeDB Lens"

// Conn holds connection settings from the CLI, GUI or a saved profile. Password never comes from argv.
type Conn struct {
	Engine          string `json:"engine"` // mssql | pg
	Server          string `json:"server"` // host, host,port or host\instance
	Port            int    `json:"port,omitempty"`
	Database        string `json:"database,omitempty"`
	Auth            string `json:"auth,omitempty"` // mssql: windows | sql
	User            string `json:"user,omitempty"`
	Password        string `json:"-"`
	DSN             string `json:"-"` // pg: full connection string, from an env var
	TrustServerCert bool   `json:"trust_server_cert,omitempty"`
	SSLMode         string `json:"sslmode,omitempty"` // pg
}

// normalize turns driver values into JSON/HTML friendly ones.
func normalize(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(x)
	case time.Time:
		return x.Format("2006-01-02 15:04:05")
	case driver.Valuer:
		if dv, err := x.Value(); err == nil {
			return normalize(dv)
		}
	case fmt.Stringer:
		return x.String()
	}
	return v
}

// Session is an open, read-only connection the engine can run rules on.
type Session interface {
	engine.Adapter
	// ReadOnly runs one statement the caller has vetted inside a transaction that is always rolled back,
	// stopping after max rows (truncated = rows were left) or at timeout. The MCP query tool's layer 2.
	ReadOnly(ctx context.Context, q string, max int, timeout time.Duration) (cols []string, rows [][]any, truncated bool, err error)
	Close() error
}

// Denied: the login lacks permission on an object (SQL Server 229/230, PostgreSQL 42501).
func Denied(err error) bool {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code == "42501"
	}
	var me interface{ SQLErrorNumber() int32 }
	if errors.As(err, &me) {
		n := me.SQLErrorNumber()
		return n == 229 || n == 230
	}
	return false
}

func Open(ctx context.Context, c Conn) (Session, error) {
	switch c.Engine {
	case "mssql":
		return OpenMSSQL(ctx, c)
	case "pg":
		return OpenPG(ctx, c)
	}
	return nil, fmt.Errorf("unknown engine %q (use mssql or pg)", c.Engine)
}
