package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/i18n"
	"github.com/codedb-io/codedb-lens/internal/profile"
)

// Limits of query (Q42/Q52): a profile may lower or raise its ceiling up to the caps.
const (
	defMaxRows  = 500
	capMaxRows  = 10000
	defTimeoutS = 10
	capTimeoutS = 120
	maxCell     = 2048 // bytes per cell: one blob must not fill the model's context
	catalogRows = 1000 // list_tables
)

func addDataTools(m *mcp.Server, s *server, ro *mcp.ToolAnnotations) {
	mcp.AddTool(m, &mcp.Tool{Name: "list_databases", Annotations: ro,
		Description: "Databases this profile's login can open: name, state, recovery model or encoding, collation and size in bytes."},
		s.listDatabases)
	mcp.AddTool(m, &mcp.Tool{Name: "list_tables", Annotations: ro,
		Description: "Tables and views visible to the profile's login in a database: schema, name, type, estimated rows, size and whether the login can read them. Start here, then describe_table, then query."},
		s.listTables)
	mcp.AddTool(m, &mcp.Tool{Name: "describe_table", Annotations: ro,
		Description: "Columns (type, nullable, default), primary key, foreign keys with the referenced table, indexes and estimated rows of one table or view. Use it before writing a query, so column names are right."},
		s.describeTable)
	mcp.AddTool(m, &mcp.Tool{Name: "query", Annotations: ro,
		Description: "Run one read-only SELECT (or WITH ... SELECT) on a profile and get columns and rows. Aggregate in SQL (GROUP BY, TOP/LIMIT) instead of fetching raw rows: results stop at the profile's row limit (500 unless the user changed it), cells at 2 KB, and truncated: true means the answer is partial. Writes, EXEC, multiple statements and functions with side effects are refused; quote a column named like an SQL keyword. SQL Server reads uncommitted data, like the health check."},
		s.query)
}

type tableOut struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"`
	Note      string   `json:"note,omitempty"` // why the result is partial or the limits changed
}

type queryOut struct {
	tableOut
	ElapsedMS int64  `json:"elapsed_ms"`
	Warning   string `json:"warning,omitempty"` // the login can write
}

type dbIn struct {
	profileIn
	Database string `json:"database,omitempty" jsonschema:"database name from list_databases; default: the profile's"`
}

type queryIn struct {
	dbIn
	SQL      string `json:"sql" jsonschema:"one SELECT or WITH ... SELECT statement"`
	MaxRows  int    `json:"max_rows,omitempty" jsonschema:"row limit; default and ceiling: the profile's"`
	TimeoutS int    `json:"timeout_s,omitempty" jsonschema:"seconds; default and ceiling: the profile's"`
}

// open connects to a profile on database db ("" = the profile's own).
func (s *server) open(ctx context.Context, in dbIn) (profile.Profile, adapter.Session, error) {
	p, err := s.profile(in.Profile)
	if err != nil {
		return p, nil, err
	}
	c, err := s.conn(p)
	if err != nil {
		return p, nil, err
	}
	if in.Database != "" {
		c.Database = in.Database
	}
	sess, err := adapter.Open(ctx, c)
	if err != nil {
		return p, nil, fmt.Errorf(i18n.T(s.lang(p), "mcp_db_open"), c.Database, err)
	}
	return p, sess, nil
}

// run is ReadOnly with errors the model can act on.
func (s *server) run(ctx context.Context, p profile.Profile, sess adapter.Session, q string, max int, timeout time.Duration) (tableOut, error) {
	cols, rows, more, err := sess.ReadOnly(ctx, q, max, timeout)
	switch {
	case err == nil:
	case adapter.Denied(err):
		return tableOut{}, fmt.Errorf(i18n.T(s.lang(p), "mcp_denied"), p.Name, err)
	case errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil:
		return tableOut{}, fmt.Errorf(i18n.T(s.lang(p), "mcp_timeout"), int(timeout.Seconds()))
	default:
		return tableOut{}, err
	}
	out := tableOut{Columns: cols, Rows: rows, Truncated: more}
	if out.Columns == nil {
		out.Columns = []string{}
	}
	if out.Rows == nil {
		out.Rows = [][]any{}
	}
	if cut(out.Rows) || more {
		out.Truncated, out.Note = true, i18n.T(s.lang(p), "mcp_truncated")
	}
	return out, nil
}

// cut trims cells over maxCell bytes on a rune boundary; reports whether any was cut.
func cut(rows [][]any) bool {
	trimmed := false
	for _, r := range rows {
		for i, v := range r {
			if s, ok := v.(string); ok && len(s) > maxCell {
				s = s[:maxCell]
				for !utf8.ValidString(s) {
					s = s[:len(s)-1]
				}
				r[i], trimmed = s+"…", true
			}
		}
	}
	return trimmed
}

// limit picks the call's value: asked (0 = default), never above the profile's ceiling, itself within cap.
func limit(asked, prof, def, cap int) (n int, capped bool) {
	ceiling := def
	if prof > 0 {
		ceiling = min(prof, cap)
	}
	if asked <= 0 {
		return ceiling, false
	}
	if asked > ceiling {
		return ceiling, true
	}
	return asked, false
}

func (s *server) query(ctx context.Context, _ *mcp.CallToolRequest, in queryIn) (*mcp.CallToolResult, queryOut, error) {
	var out queryOut
	p, err := s.profile(in.Profile)
	if err != nil {
		return nil, out, err
	}
	lang := s.lang(p)
	if word, ok := guard(p.Conn.Engine, in.SQL); !ok {
		if word != "" {
			return nil, out, fmt.Errorf(i18n.T(lang, "mcp_query_word"), word)
		}
		return nil, out, errors.New(i18n.T(lang, "mcp_query_shape"))
	}
	rows, rowsCapped := limit(in.MaxRows, p.MCPMaxRows, defMaxRows, capMaxRows)
	secs, secsCapped := limit(in.TimeoutS, p.MCPTimeoutS, defTimeoutS, capTimeoutS)
	p, sess, err := s.open(ctx, in.dbIn)
	if err != nil {
		return nil, out, err
	}
	defer sess.Close()
	// Layer 4: the login is the real barrier, so say when it is not one. Before the query: a truncated
	// result cancels it, and the session goes with it.
	q := `SELECT CAST(IS_SRVROLEMEMBER('sysadmin') | ISNULL(IS_MEMBER('db_owner'), 0) AS bit)`
	if p.Conn.Engine == "pg" {
		q = `SELECT rolsuper FROM pg_roles WHERE rolname = current_user`
	}
	if t, err := s.run(ctx, p, sess, q, 1, 5*time.Second); err == nil && len(t.Rows) == 1 && truthy(t.Rows[0][0]) {
		out.Warning = i18n.T(lang, "mcp_login_writes")
	}
	start := time.Now()
	if out.tableOut, err = s.run(ctx, p, sess, in.SQL, rows, time.Duration(secs)*time.Second); err != nil {
		return nil, out, err
	}
	out.ElapsedMS = time.Since(start).Milliseconds()
	if rowsCapped || secsCapped {
		out.Note = strings.TrimSpace(out.Note + " " + fmt.Sprintf(i18n.T(lang, "mcp_capped"), rows, secs))
	}
	return nil, out, nil
}

// catalog runs one of the schema tools' own queries, picked by engine.
func (s *server) catalog(ctx context.Context, in dbIn, sql func(eng, platform string) string) (tableOut, error) {
	p, sess, err := s.open(ctx, in)
	if err != nil {
		return tableOut{}, err
	}
	defer sess.Close()
	return s.run(ctx, p, sess, sql(p.Conn.Engine, sess.Info().Platform), catalogRows, defTimeoutS*time.Second)
}

func (s *server) listDatabases(ctx context.Context, _ *mcp.CallToolRequest, in profileIn) (*mcp.CallToolResult, tableOut, error) {
	out, err := s.catalog(ctx, dbIn{profileIn: in}, func(eng, platform string) string {
		switch {
		case eng == "pg":
			return `SELECT datname AS name, pg_encoding_to_char(encoding) AS encoding, datcollate AS collation,
				pg_database_size(oid) AS size_bytes
				FROM pg_database WHERE datallowconn AND NOT datistemplate AND has_database_privilege(oid, 'CONNECT')
				ORDER BY datname`
		case platform == "azure_sqldb": // one database per connection, no sys.master_files
			return `SELECT DB_NAME() AS name, 'ONLINE' AS state, NULL AS recovery_model,
				CAST(DATABASEPROPERTYEX(DB_NAME(), 'Collation') AS sysname) AS collation,
				(SELECT SUM(CAST(size AS bigint)) * 8192 FROM sys.database_files) AS size_bytes`
		}
		return `SELECT d.name, d.state_desc AS state, d.recovery_model_desc AS recovery_model, d.collation_name AS collation,
			(SELECT SUM(CAST(f.size AS bigint)) * 8192 FROM sys.master_files f WHERE f.database_id = d.database_id) AS size_bytes
			FROM sys.databases d WHERE d.database_id > 4 AND HAS_DBACCESS(d.name) = 1 ORDER BY d.name`
	})
	return nil, out, err
}

type listTablesIn struct {
	dbIn
	Schema string `json:"schema,omitempty" jsonschema:"only this schema"`
	Filter string `json:"filter,omitempty" jsonschema:"name pattern: plain text matches anywhere in the name, or a LIKE pattern with %"`
}

// lit is a string literal for eng; the name parts of the schema tools are the only text spliced into SQL.
func lit(eng, s string) string {
	q := "'" + strings.ReplaceAll(s, "'", "''") + "'"
	if eng == "mssql" {
		return "N" + q
	}
	return q
}

func (s *server) listTables(ctx context.Context, _ *mcp.CallToolRequest, in listTablesIn) (*mcp.CallToolResult, tableOut, error) {
	out, err := s.catalog(ctx, in.dbIn, func(eng, _ string) string {
		where := ""
		like := in.Filter
		if like != "" && !strings.Contains(like, "%") {
			like = "%" + like + "%"
		}
		if eng == "pg" {
			if in.Schema != "" {
				where += " AND n.nspname = " + lit(eng, in.Schema)
			}
			if like != "" {
				where += " AND c.relname ILIKE " + lit(eng, like)
			}
			return `SELECT n.nspname AS schema, c.relname AS name,
				CASE c.relkind WHEN 'v' THEN 'view' WHEN 'm' THEN 'materialized view' WHEN 'f' THEN 'foreign table' ELSE 'table' END AS type,
				CASE WHEN c.reltuples >= 0 THEN c.reltuples::bigint END AS estimated_rows,
				CASE WHEN c.relkind IN ('r', 'p', 'm') THEN pg_total_relation_size(c.oid) END AS size_bytes,
				has_table_privilege(c.oid, 'SELECT') AS readable
				FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f') AND NOT c.relispartition
				AND n.nspname NOT IN ('pg_catalog', 'information_schema') AND n.nspname NOT LIKE 'pg\_toast%'
				AND n.nspname NOT LIKE 'pg\_temp%'` + where + ` ORDER BY 1, 2`
		}
		if in.Schema != "" {
			where += " AND s.name = " + lit(eng, in.Schema)
		}
		if like != "" {
			where += " AND o.name LIKE " + lit(eng, like)
		}
		return `SELECT s.name AS [schema], o.name, CASE o.type WHEN 'U' THEN 'table' ELSE 'view' END AS type,
			(SELECT SUM(p.rows) FROM sys.partitions p WHERE p.object_id = o.object_id AND p.index_id IN (0, 1)) AS estimated_rows,
			(SELECT SUM(a.total_pages) * 8192 FROM sys.partitions p JOIN sys.allocation_units a ON a.container_id = p.partition_id
				WHERE p.object_id = o.object_id) AS size_bytes,
			CAST(HAS_PERMS_BY_NAME(QUOTENAME(s.name) + '.' + QUOTENAME(o.name), 'OBJECT', 'SELECT') AS bit) AS readable
			FROM sys.objects o JOIN sys.schemas s ON s.schema_id = o.schema_id
			WHERE o.type IN ('U', 'V') AND o.is_ms_shipped = 0` + where + ` ORDER BY 1, 2`
	})
	return nil, out, err
}

type describeIn struct {
	dbIn
	Table string `json:"table" jsonschema:"table or view: schema.name, or name in the default schema"`
}

type columnOut struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
}

type indexOut struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary"`
	Columns []string `json:"columns"`
}

type foreignKeyOut struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	References string   `json:"references"`
	RefColumns []string `json:"ref_columns"`
}

type describeOut struct {
	Schema        string          `json:"schema"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	EstimatedRows *int64          `json:"estimated_rows,omitempty"`
	Columns       []columnOut     `json:"columns"`
	PrimaryKey    []string        `json:"primary_key"`
	ForeignKeys   []foreignKeyOut `json:"foreign_keys"`
	Indexes       []indexOut      `json:"indexes"`
}

// Catalog queries of describe_table. %s is the table literal in resolve, %d its object id in the others.
var describeSQL = map[string][4]string{
	"mssql": {
		`SELECT o.object_id, s.name, o.name, CASE o.type WHEN 'U' THEN 'table' ELSE 'view' END,
			(SELECT SUM(p.rows) FROM sys.partitions p WHERE p.object_id = o.object_id AND p.index_id IN (0, 1))
			FROM sys.objects o JOIN sys.schemas s ON s.schema_id = o.schema_id
			WHERE o.object_id = OBJECT_ID(%s) AND o.type IN ('U', 'V')`,
		`SELECT c.name, TYPE_NAME(c.user_type_id) + CASE
				WHEN TYPE_NAME(c.user_type_id) IN ('varchar', 'char', 'varbinary', 'binary')
					THEN '(' + CASE WHEN c.max_length = -1 THEN 'max' ELSE CAST(c.max_length AS varchar(10)) END + ')'
				WHEN TYPE_NAME(c.user_type_id) IN ('nvarchar', 'nchar')
					THEN '(' + CASE WHEN c.max_length = -1 THEN 'max' ELSE CAST(c.max_length / 2 AS varchar(10)) END + ')'
				WHEN TYPE_NAME(c.user_type_id) IN ('decimal', 'numeric')
					THEN '(' + CAST(c.precision AS varchar(3)) + ',' + CAST(c.scale AS varchar(3)) + ')'
				ELSE '' END,
			c.is_nullable, dc.definition
			FROM sys.columns c LEFT JOIN sys.default_constraints dc ON dc.object_id = c.default_object_id
			WHERE c.object_id = %d ORDER BY c.column_id`,
		`SELECT i.name, LOWER(i.type_desc), i.is_unique, i.is_primary_key, c.name
			FROM sys.indexes i
			JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id AND ic.is_included_column = 0
			JOIN sys.columns c ON c.object_id = ic.object_id AND c.column_id = ic.column_id
			WHERE i.object_id = %d ORDER BY i.index_id, ic.key_ordinal, ic.index_column_id`,
		`SELECT fk.name, pc.name, OBJECT_SCHEMA_NAME(fk.referenced_object_id) + '.' + OBJECT_NAME(fk.referenced_object_id), rc.name
			FROM sys.foreign_keys fk
			JOIN sys.foreign_key_columns fkc ON fkc.constraint_object_id = fk.object_id
			JOIN sys.columns pc ON pc.object_id = fkc.parent_object_id AND pc.column_id = fkc.parent_column_id
			JOIN sys.columns rc ON rc.object_id = fkc.referenced_object_id AND rc.column_id = fkc.referenced_column_id
			WHERE fk.parent_object_id = %d ORDER BY fk.name, fkc.constraint_column_id`,
	},
	"pg": {
		`SELECT c.oid::bigint, n.nspname, c.relname,
			CASE c.relkind WHEN 'v' THEN 'view' WHEN 'm' THEN 'materialized view' WHEN 'f' THEN 'foreign table' ELSE 'table' END,
			CASE WHEN c.reltuples >= 0 THEN c.reltuples::bigint END
			FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE c.oid = to_regclass(%s) AND c.relkind IN ('r', 'p', 'v', 'm', 'f')`,
		`SELECT a.attname, format_type(a.atttypid, a.atttypmod), NOT a.attnotnull, pg_get_expr(d.adbin, d.adrelid)
			FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
			WHERE a.attrelid = %d AND a.attnum > 0 AND NOT a.attisdropped ORDER BY a.attnum`,
		`SELECT ic.relname, am.amname, i.indisunique, i.indisprimary, COALESCE(a.attname, '(expression)')
			FROM pg_index i JOIN pg_class ic ON ic.oid = i.indexrelid JOIN pg_am am ON am.oid = ic.relam
			CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord)
			LEFT JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
			WHERE i.indrelid = %d AND k.ord <= i.indnkeyatts ORDER BY ic.relname, k.ord`,
		`SELECT con.conname, a.attname, con.confrelid::regclass::text, ra.attname
			FROM pg_constraint con
			CROSS JOIN LATERAL unnest(con.conkey, con.confkey) WITH ORDINALITY AS k(col, refcol, ord)
			JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.col
			JOIN pg_attribute ra ON ra.attrelid = con.confrelid AND ra.attnum = k.refcol
			WHERE con.conrelid = %d AND con.contype = 'f' ORDER BY con.conname, k.ord`,
	},
}

func (s *server) describeTable(ctx context.Context, _ *mcp.CallToolRequest, in describeIn) (*mcp.CallToolResult, describeOut, error) {
	out := describeOut{Columns: []columnOut{}, PrimaryKey: []string{}, ForeignKeys: []foreignKeyOut{}, Indexes: []indexOut{}}
	p, sess, err := s.open(ctx, in.dbIn)
	if err != nil {
		return nil, out, err
	}
	defer sess.Close()
	eng := p.Conn.Engine
	sql := describeSQL[eng]
	get := func(q string) ([][]any, error) {
		t, err := s.run(ctx, p, sess, q, catalogRows, defTimeoutS*time.Second)
		return t.Rows, err
	}
	rows, err := get(fmt.Sprintf(sql[0], lit(eng, in.Table)))
	if err != nil {
		return nil, out, err
	}
	if len(rows) == 0 {
		return nil, out, fmt.Errorf(i18n.T(s.lang(p), "mcp_table_unknown"), in.Table)
	}
	id, _ := toInt(rows[0][0])
	out.Schema, out.Name, out.Type = str(rows[0][1]), str(rows[0][2]), str(rows[0][3])
	if n, ok := toInt(rows[0][4]); ok {
		out.EstimatedRows = &n
	}
	if rows, err = get(fmt.Sprintf(sql[1], id)); err != nil {
		return nil, out, err
	}
	for _, r := range rows {
		out.Columns = append(out.Columns, columnOut{str(r[0]), str(r[1]), truthy(r[2]), str(r[3])})
	}
	if rows, err = get(fmt.Sprintf(sql[2], id)); err != nil {
		return nil, out, err
	}
	for _, r := range rows { // one row per key column, grouped by index
		if n := len(out.Indexes); n == 0 || out.Indexes[n-1].Name != str(r[0]) {
			out.Indexes = append(out.Indexes, indexOut{str(r[0]), str(r[1]), truthy(r[2]), truthy(r[3]), nil})
		}
		ix := &out.Indexes[len(out.Indexes)-1]
		ix.Columns = append(ix.Columns, str(r[4]))
		if ix.Primary {
			out.PrimaryKey = ix.Columns
		}
	}
	if rows, err = get(fmt.Sprintf(sql[3], id)); err != nil {
		return nil, out, err
	}
	for _, r := range rows {
		if n := len(out.ForeignKeys); n == 0 || out.ForeignKeys[n-1].Name != str(r[0]) {
			out.ForeignKeys = append(out.ForeignKeys, foreignKeyOut{Name: str(r[0]), References: str(r[2])})
		}
		fk := &out.ForeignKeys[len(out.ForeignKeys)-1]
		fk.Columns, fk.RefColumns = append(fk.Columns, str(r[1])), append(fk.RefColumns, str(r[3]))
	}
	return nil, out, nil
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func toInt(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int32:
		return int64(x), true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	}
	return 0, false
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int64:
		return x != 0
	}
	return false
}
