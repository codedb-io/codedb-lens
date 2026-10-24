//go:build integration

// The MCP data tools against the docker-compose databases (plan §7.3, §7.3.1): the lens_shop fixture read by a
// "data profile" login, the limits, and the attack corpus run by a login that can write, with a canary.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"

	"github.com/codedb-io/codedb-lens/internal/adapter"
	"github.com/codedb-io/codedb-lens/internal/mcpserver"
	"github.com/codedb-io/codedb-lens/internal/profile"
)

const dataPassword = "Lens_data_Passw0rd!"

// dataFixture describes one engine's setup for mcpData.
type dataFixture struct {
	admin, data adapter.Conn  // admin can write; data reads lens_shop except secret
	canary      func() string // object count and a checksum of customers: must not change
	rows501     string
	bigCell     string
	slow        string
	refused     []string // the guard corpus for this engine
	direct      string   // a statement without a transaction, for the write mode
	secretRows  func() int64
}

func mcpData(t *testing.T, f dataFixture) {
	dir := t.TempDir()
	for _, k := range []string{"APPDATA", "XDG_CONFIG_HOME", "USERPROFILE", "HOME"} {
		t.Setenv(k, dir)
	}
	keyring.MockInit()
	pf := profile.File{}
	pf.Put(profile.Profile{Name: "admin", Conn: f.admin})
	pf.Put(profile.Profile{Name: "data", Conn: f.data})
	if err := pf.Save(); err != nil {
		t.Fatal(err)
	}
	_ = profile.SetPassword("admin", f.admin.Password)
	_ = profile.SetPassword("data", dataPassword)

	srv, err := mcpserver.New(mcpOptions("en", "", false))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, cl := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, cl, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	// call returns the structured result, or the tool error's text.
	call := func(name string, args map[string]any) (map[string]any, string) {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.IsError {
			return nil, res.Content[0].(*mcp.TextContent).Text
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		return m, ""
	}
	must := func(name string, args map[string]any) map[string]any {
		t.Helper()
		m, e := call(name, args)
		if e != "" {
			t.Fatalf("%s %v: %s", name, args, e)
		}
		return m
	}
	data := func(extra map[string]any) map[string]any {
		a := map[string]any{"profile": "data", "database": "lens_shop"}
		for k, v := range extra {
			a[k] = v
		}
		return a
	}

	// Schema, as the data login sees it.
	if !strings.Contains(fmt.Sprint(must("list_databases", map[string]any{"profile": "data"})["rows"]), "lens_shop") {
		t.Error("list_databases: no lens_shop")
	}
	tables := map[string]bool{} // name -> readable
	for _, r := range must("list_tables", data(nil))["rows"].([]any) {
		row := r.([]any)
		tables[row[1].(string)] = row[5] == true
	}
	for _, name := range []string{"customers", "orders", "v_revenue"} {
		if !tables[name] {
			t.Errorf("list_tables: %s missing or not readable: %v", name, tables)
		}
	}
	if tables["secret"] {
		t.Errorf("list_tables: secret readable by the data login")
	}
	d := must("describe_table", data(map[string]any{"table": "orders"}))
	if fmt.Sprint(d["primary_key"]) != "[id]" || !strings.Contains(fmt.Sprint(d["columns"]), "customer_id") {
		t.Errorf("describe_table orders: %v", d)
	}
	if fks := d["foreign_keys"].([]any); len(fks) != 1 || !strings.HasSuffix(fks[0].(map[string]any)["references"].(string), "customers") {
		t.Errorf("describe_table orders: foreign keys %v", d["foreign_keys"])
	}
	if !strings.Contains(fmt.Sprint(d["indexes"]), "ix_orders_customer") {
		t.Errorf("describe_table orders: indexes %v", d["indexes"])
	}
	if _, e := call("describe_table", data(map[string]any{"table": "nope"})); !strings.Contains(e, "not found") {
		t.Errorf("describe_table nope: %q", e)
	}

	// Data questions.
	q := must("query", data(map[string]any{"sql": `SELECT c.city, SUM(o.total) AS revenue FROM customers c
		JOIN orders o ON o.customer_id = c.id GROUP BY c.city ORDER BY c.city`}))
	if got := fmt.Sprint(q["rows"]); got != "[[Recife 30] [Santos 5]]" {
		t.Errorf("revenue by city: %s", got)
	}
	if q["warning"] != nil || q["truncated"] != false {
		t.Errorf("data login query: %v", q)
	}
	if _, e := call("query", data(map[string]any{"sql": "SELECT * FROM secret"})); !strings.Contains(e, "cannot read") {
		t.Errorf("secret: %q", e)
	}
	if _, e := call("query", map[string]any{"profile": "data", "database": "no_such_db", "sql": "SELECT 1"}); !strings.Contains(e, "could not open database") {
		t.Errorf("missing database: %q", e)
	}

	// Limits, with the login that can write (it gets the warning).
	admin := func(sql string, extra ...any) (map[string]any, string) {
		a := map[string]any{"profile": "admin", "database": "lens_shop", "sql": sql}
		for i := 0; i < len(extra); i += 2 {
			a[extra[i].(string)] = extra[i+1]
		}
		return call("query", a)
	}
	m, e := admin(f.rows501)
	if e != "" || len(m["rows"].([]any)) != 500 || m["truncated"] != true || !strings.Contains(m["note"].(string), "partial") {
		t.Errorf("501 rows: %d %v %v %s", len(m["rows"].([]any)), m["truncated"], m["note"], e)
	}
	if m["warning"] == nil {
		t.Error("no warning for a login that can write")
	}
	if m, e = admin(f.rows501, "max_rows", 20000); e != "" || len(m["rows"].([]any)) != 500 || !strings.Contains(m["note"].(string), "lowered") {
		t.Errorf("max_rows above the ceiling: %v %s", m["note"], e)
	}
	if m, e = admin(f.bigCell); e != "" || len(m["rows"].([]any)[0].([]any)[0].(string)) > 2048+len("…") || m["truncated"] != true {
		t.Errorf("10 KB cell: %s", e)
	}
	start := time.Now()
	if _, e = admin(f.slow, "timeout_s", 1); !strings.Contains(e, "longer than 1 s") || time.Since(start) > 8*time.Second {
		t.Errorf("timeout: %q after %v", e, time.Since(start))
	}

	// The corpus: every case refused, nothing changed.
	before := f.canary()
	for _, sql := range f.refused {
		if _, e := admin(sql); e == "" {
			t.Errorf("allowed: %q", sql)
		}
	}
	// Layer 2 alone: a write that skips the guard is undone (SQL Server) or refused by the server (PostgreSQL).
	c := f.admin
	c.Database = "lens_shop"
	sess, err := adapter.Open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = sess.ReadOnly(ctx, "DELETE FROM customers", 10, 5*time.Second)
	sess.Close()
	if c.Engine == "pg" && (err == nil || !strings.Contains(err.Error(), "read-only")) {
		t.Errorf("pg write in ReadOnly: %v", err)
	}
	if after := f.canary(); after != before {
		t.Fatalf("canary changed: %s -> %s", before, after)
	}

	mcpWrite(t, f, before)
}

// mcpWrite: the write mode with both keys on and a user who confirms (plan §7.6): a preview counts the rows and
// leaves the database as it was, dry_run false applies, and every run is in the audit log.
func mcpWrite(t *testing.T, f dataFixture, canary string) {
	pf, _ := profile.Load()
	p, _ := pf.Get("admin")
	p.MCPWrite = true
	pf.Put(p)
	if err := pf.Save(); err != nil {
		t.Fatal(err)
	}
	o := mcpOptions("en", "", false)
	o.AllowWrite = "confirm"
	srv, err := mcpserver.New(o)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, cl := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	asked := 0
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, &mcp.ClientOptions{
		ElicitationHandler: func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			asked++
			return &mcp.ElicitResult{Action: "accept", Content: map[string]any{"confirm": true}}, nil
		}}).Connect(ctx, cl, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	run := func(sql string, dry bool) map[string]any {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "execute_sql",
			Arguments: map[string]any{"profile": "admin", "database": "lens_shop", "sql": sql, "dry_run": dry}})
		if err != nil || res.IsError {
			t.Fatalf("execute_sql %q: %v %+v", sql, err, res)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		return m
	}
	if m := run("UPDATE customers SET city = 'X'", true); m["rows_affected"] != float64(2) || m["rolled_back"] != true {
		t.Errorf("preview: %v", m)
	}
	if after := f.canary(); after != canary {
		t.Fatalf("preview changed the canary: %s -> %s", canary, after)
	}
	before := f.secretRows()
	if m := run("INSERT INTO secret VALUES (1)", false); m["rows_affected"] != float64(1) || m["rolled_back"] != false {
		t.Errorf("apply: %v", m)
	}
	if n := f.secretRows(); n != before+1 {
		t.Errorf("apply: secret has %d rows, want %d", n, before+1)
	}
	run(f.direct, false)

	// A COMMIT inside a preview (security review): PostgreSQL takes one statement only; SQL Server cannot stop a
	// batch from committing, so Write reports it and execute_sql's deny list keeps COMMIT out.
	c := f.admin
	c.Database = "lens_shop"
	sess, err := adapter.Open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	before = f.secretRows()
	_, err = sess.Write(ctx, "INSERT INTO secret VALUES (2); COMMIT", adapter.Preview, 5*time.Second)
	sess.Close()
	if c.Engine == "pg" && (err == nil || f.secretRows() != before) {
		t.Errorf("pg preview with COMMIT: err %v, rows %d -> %d", err, before, f.secretRows())
	}
	if c.Engine == "mssql" && !errors.Is(err, adapter.ErrTxEnded) {
		t.Errorf("mssql preview with COMMIT: want ErrTxEnded, got %v", err)
	}
	if res, _ := cs.CallTool(ctx, &mcp.CallToolParams{Name: "execute_sql", Arguments: map[string]any{
		"profile": "admin", "database": "lens_shop", "sql": "INSERT INTO secret VALUES (3); COMMIT"}}); res == nil || !res.IsError {
		t.Error("execute_sql ran a statement with COMMIT")
	}
	if asked != 3 {
		t.Errorf("the user was asked %d times, want 3", asked)
	}
	path, _ := mcpserver.AuditPath()
	raw, _ := os.ReadFile(path)
	if a := string(raw); strings.Count(a, `"result":"ok"`) != 3 || !strings.Contains(a, `"mode":"preview"`) || !strings.Contains(a, `"mode":"direct"`) {
		t.Errorf("audit:\n%s", a)
	}
}

// refusedCorpus is the guard's attack corpus for engine (internal/mcpserver/guard_test.go).
func refusedCorpus(t *testing.T, engine string) []string {
	raw, err := os.ReadFile("../../internal/mcpserver/testdata/guard_corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var c struct{ Refused map[string][]string }
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return append(c.Refused["both"], c.Refused[engine]...)
}

func TestMSSQLMCPData(t *testing.T) {
	server, pw := envOr("LENS_MSSQL_SERVER", "localhost,14322"), envOr("MSSQL_SA_PASSWORD", "Lens_dev_Passw0rd!")
	admin, _ := sql.Open("sqlserver", "sqlserver://sa:"+pw+"@"+strings.Replace(server, ",", ":", 1)+"?encrypt=true&TrustServerCertificate=true")
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		skipOrFail(t, "SQL Server not reachable:", err)
	}
	exec := func(db, q string) {
		t.Helper()
		if _, err := admin.Exec("USE " + db + "; " + q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	drop := func() {
		_, _ = admin.Exec(`IF DB_ID('lens_shop') IS NOT NULL BEGIN ALTER DATABASE lens_shop SET SINGLE_USER WITH ROLLBACK IMMEDIATE; DROP DATABASE lens_shop; END;
			IF SUSER_ID('lens_data') IS NOT NULL DROP LOGIN lens_data`)
	}
	drop()
	t.Cleanup(drop)
	exec("master", "CREATE DATABASE lens_shop; CREATE LOGIN lens_data WITH PASSWORD = '"+dataPassword+"', CHECK_POLICY = OFF")
	exec("lens_shop", `CREATE TABLE customers (id int PRIMARY KEY, name nvarchar(50), city nvarchar(50));
		CREATE TABLE orders (id int PRIMARY KEY, customer_id int NOT NULL REFERENCES customers (id), total decimal(10, 0));
		CREATE INDEX ix_orders_customer ON orders (customer_id);
		CREATE TABLE secret (id int);
		INSERT customers VALUES (1, 'Ana', 'Recife'), (2, 'Bia', 'Santos');
		INSERT orders VALUES (1, 1, 10), (2, 1, 20), (3, 2, 5);`)
	exec("lens_shop", "EXEC('CREATE VIEW v_revenue AS SELECT customer_id, SUM(total) AS total FROM orders GROUP BY customer_id')")
	exec("lens_shop", `CREATE USER lens_data FOR LOGIN lens_data; ALTER ROLE db_datareader ADD MEMBER lens_data;
		DENY SELECT ON secret TO lens_data;`)
	conn := adapter.Conn{Engine: "mssql", Server: server, User: "sa", Password: pw, TrustServerCert: true}
	data := conn
	data.User, data.Password, data.Database = "lens_data", "", "lens_shop"
	mcpData(t, dataFixture{admin: conn, data: data,
		canary: func() string {
			var n, sum int64
			_ = admin.QueryRow(`SELECT (SELECT COUNT(*) FROM lens_shop.sys.objects), (SELECT CHECKSUM_AGG(CHECKSUM(*)) FROM lens_shop.dbo.customers)`).Scan(&n, &sum)
			return strconv.FormatInt(n, 10) + "/" + strconv.FormatInt(sum, 10)
		},
		rows501: "SELECT TOP 501 ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) AS n FROM sys.all_objects a CROSS JOIN sys.all_objects b",
		bigCell: "SELECT REPLICATE(CAST('x' AS varchar(max)), 10000) AS s",
		slow:    "SELECT COUNT_BIG(*) FROM sys.all_objects a CROSS JOIN sys.all_objects b CROSS JOIN sys.all_objects c WHERE CAST(a.object_id AS bigint) + b.object_id + c.object_id = 7",
		refused: refusedCorpus(t, "mssql"),
		direct:  "ALTER DATABASE lens_shop SET RECOVERY SIMPLE",
		secretRows: func() (n int64) {
			_ = admin.QueryRow("SELECT COUNT_BIG(*) FROM lens_shop.dbo.secret").Scan(&n)
			return n
		},
	})
}

func TestPGMCPData(t *testing.T) {
	dsn := envOr("LENS_PG_DSN", "postgres://postgres:lens_dev@localhost:15417/postgres?sslmode=disable")
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		skipOrFail(t, "PostgreSQL not reachable:", err)
	}
	defer admin.Close(ctx)
	cfg := admin.Config()
	var shop *pgx.Conn
	exec := func(db, q string) {
		t.Helper()
		c := admin
		if db == "lens_shop" {
			c = shop
		}
		if _, err := c.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	drop := func() {
		if shop != nil {
			shop.Close(ctx)
		}
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS lens_shop WITH (FORCE)")
		_, _ = admin.Exec(ctx, "DROP ROLE IF EXISTS lens_data")
	}
	drop()
	t.Cleanup(drop)
	exec("postgres", "CREATE DATABASE lens_shop")
	exec("postgres", "CREATE ROLE lens_data LOGIN PASSWORD '"+dataPassword+"'")
	sc := cfg.Copy()
	sc.Database = "lens_shop"
	if shop, err = pgx.ConnectConfig(ctx, sc); err != nil {
		t.Fatal(err)
	}
	exec("lens_shop", `CREATE TABLE customers (id int PRIMARY KEY, name text, city text);
		CREATE TABLE orders (id int PRIMARY KEY, customer_id int NOT NULL REFERENCES customers (id), total numeric(10, 0));
		CREATE INDEX ix_orders_customer ON orders (customer_id);
		CREATE TABLE secret (id int);
		CREATE VIEW v_revenue AS SELECT customer_id, SUM(total) AS total FROM orders GROUP BY customer_id;
		INSERT INTO customers VALUES (1, 'Ana', 'Recife'), (2, 'Bia', 'Santos');
		INSERT INTO orders VALUES (1, 1, 10), (2, 1, 20), (3, 2, 5);
		GRANT SELECT ON customers, orders, v_revenue TO lens_data;
		ANALYZE;`)
	conn := adapter.Conn{Engine: "pg", Server: cfg.Host, Port: int(cfg.Port), User: cfg.User, Password: cfg.Password, SSLMode: "disable"}
	data := conn
	data.User, data.Password, data.Database = "lens_data", "", "lens_shop"
	mcpData(t, dataFixture{admin: conn, data: data,
		canary: func() string {
			var n int64
			var sum string
			_ = shop.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_class), (SELECT md5(string_agg(c::text, ',' ORDER BY id)) FROM customers c)`).Scan(&n, &sum)
			return strconv.FormatInt(n, 10) + "/" + sum
		},
		rows501: "SELECT n FROM generate_series(1, 501) AS n",
		bigCell: "SELECT repeat('x', 10000) AS s",
		slow:    "SELECT count(*) FROM generate_series(1, 2000000000)",
		refused: refusedCorpus(t, "pg"),
		direct:  "VACUUM customers",
		secretRows: func() (n int64) {
			_ = shop.QueryRow(ctx, "SELECT count(*) FROM secret").Scan(&n)
			return n
		},
	})
}
