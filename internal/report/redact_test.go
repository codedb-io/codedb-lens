package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codedb-io/codedb-lens/internal/engine"
)

func evidence(id string, cols []string, rows ...[]any) engine.Result {
	r := finding(id, engine.High, engine.Confirmed)
	r.Columns, r.Rows = cols, rows
	return r
}

func secretReport() *Report {
	r := &Report{Tool: "codeDB Lens", ToolVersion: "test", GeneratedAt: time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC), Lang: "en",
		Server: engine.ServerInfo{Engine: "mssql", Host: "prod-sql-01.acme.corp"}}
	r.Results = []engine.Result{
		evidence("MSSQL-BACKUP-001", []string{"database", "recovery_model"}, []any{"AcmeBilling", "FULL"}, []any{"master", "SIMPLE"}),
		evidence("MSSQL-BLOCKING-001", []string{"blocked_session", "database", "blocker_login", "blocker_host", "blocker_program", "blocked_query"},
			[]any{55, "AcmeBilling", `acme\jsilva`, "WKS-JSILVA", "BillingApp v2", "UPDATE invoices SET x = 'secret-literal'"}),
		evidence("MSSQL-UNUSEDIDX-001", []string{"database", "table", "index", "keys", "duplicate_of", "reads"},
			[]any{"AcmeBilling", "dbo.Invoices", "IX_Invoices_Customer", "CustomerCpf, Total DESC", "IX_Invoices_Customer2", 0}),
		evidence("MSSQL-MISSINGIDX-001", []string{"database", "table", "equality_columns", "inequality_columns", "included_columns"},
			[]any{"AcmeBilling", "[AcmeBilling].[dbo].[Payments]", "[CardNumber]", "[PaidAt]", "[HolderName]"}),
		evidence("MSSQL-SPACE-001", []string{"kind", "name", "free_pct"}, []any{"volume", `E:\AcmeData\`, 4.1}, []any{"log", "AcmeBilling", nil}),
		evidence("MSSQL-TEMPDB-001", []string{"issue", "file", "current_value"}, []any{"growth in percent", "acme_tempdev2", "10%"}),
		evidence("MSSQL-AGENT-001", []string{"job", "outcome", "message"}, []any{"Acme nightly ETL", "failed", "Login failed for user 'etl_acme'"}),
		evidence("MSSQL-TOPQUERY-001", []string{"cpu_pct", "database", "query"}, []any{55.5, "AcmeBilling", "SELECT * FROM Customers WHERE cpf = '123.456.789-00'"}),
		evidence("PG-LOCKS-001", []string{"blocked_pid", "database", "blocker_user", "blocker_application", "blocked_query"},
			[]any{10, "acme_erp", "erp_writer", "acme-erp-worker", "DELETE FROM acme_orders WHERE id = 7"}),
		evidence("PG-LONGTX-001", []string{"pid", "database", "user", "application_name", "client_addr", "last_query"},
			[]any{11, "acme_erp", "erp_writer", "acme-erp-worker", "10.20.30.40", "SELECT secret_col FROM acme_orders"}),
		evidence("PG-SLOTS-001", []string{"kind", "name", "type"}, []any{"slot", "acme_debezium_slot", "logical"}, []any{"replica", "standby-acme-02", "physical"}),
		evidence("PG-SIZE-001", []string{"database", "relation", "total_gb"}, []any{"acme_erp", "sales.acme_orders", 12.5}),
		evidence("PG-TOPQUERY-001", []string{"time_pct", "database", "query"}, []any{70.0, "acme_erp", []byte("SELECT card FROM acme_cards")}),
		{Rule: &engine.Rule{ID: "MSSQL-CHECKDB-001", Severity: engine.Medium, Title: engine.Text{"en": "x", "pt-BR": "x"}}, Status: engine.StatusNoData,
			Reason: "skipped databases: HiddenLegacyDb (no access); AcmeBilling (cannot open database \"AcmeBilling\" for host prod-sql-01.acme.corp)"},
	}
	return r
}

var secrets = []string{"prod-sql-01", "AcmeBilling", "jsilva", "WKS-JSILVA", "BillingApp", "secret-literal", "Invoices", "CustomerCpf", "IX_Invoices",
	"CardNumber", "PaidAt", "HolderName", "AcmeData", "acme_tempdev2", "Acme nightly", "etl_acme", "123.456.789-00", "acme_erp", "erp_writer",
	"acme-erp-worker", "acme_orders", "10.20.30.40", "acme_debezium_slot", "standby-acme-02", "acme_cards", "HiddenLegacyDb", "secret_col", "Payments"}

func render(t *testing.T, r *Report) string {
	t.Helper()
	var h, j bytes.Buffer
	if err := r.HTML(&h); err != nil {
		t.Fatal(err)
	}
	if err := r.JSON(&j); err != nil {
		t.Fatal(err)
	}
	return h.String() + j.String()
}

func TestRedact(t *testing.T) {
	salt := bytes.Repeat([]byte{7}, 32)
	if out := render(t, secretReport()); !strings.Contains(out, "AcmeBilling") {
		t.Fatal("test setup: secrets must be in the unmasked report")
	}

	r := secretReport()
	r.Redact(salt)
	out := render(t, r)
	for _, s := range secrets {
		if strings.Contains(out, s) {
			t.Errorf("redacted report still contains %q", s)
		}
	}
	for _, must := range []string{"master", "FULL", "failed", "growth in percent", "10%", "Redacted: names masked", `"redacted": true`,
		"host-", "db-", "obj-", "login-", "app-", "addr-", "job-", "slot-", "path-", "[redacted]"} {
		if !strings.Contains(out, must) {
			t.Errorf("redacted report lacks %q", must)
		}
	}
	if got := r.Results[0].Rows[0][0]; r.Results[4].Rows[1][1] != got { // same name, same kind: same token in another rule
		t.Errorf("AcmeBilling tokens differ: %v vs %v", got, r.Results[4].Rows[1][1])
	}
	if r.Results[0].Rows[1][0] != "master" {
		t.Error("system database must stay visible")
	}

	again := secretReport()
	again.Redact(salt)
	if again.Server.Host != r.Server.Host {
		t.Error("same salt must give the same token")
	}
	other := secretReport()
	other.Redact(bytes.Repeat([]byte{8}, 32))
	if other.Server.Host == r.Server.Host {
		t.Error("another salt must give another token")
	}
}

// Redacted runs are saved as .redacted.* only and chain with each other, never with plain runs.
func TestRedactSaveAndEvolution(t *testing.T) {
	dir, salt := t.TempDir(), bytes.Repeat([]byte{7}, 32)
	prev := secretReport()
	prev.Redact(salt)
	if p, err := prev.Save(dir); err != nil || !strings.HasSuffix(p, ".redacted.html") {
		t.Fatalf("save: %v %v", p, err)
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "*")); len(files) != 2 {
		t.Errorf("want only the two redacted files, got %v", files)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-10-22_100000.json")); err == nil {
		t.Error("unmasked json written")
	}

	cur := secretReport()
	cur.GeneratedAt = cur.GeneratedAt.Add(time.Hour)
	cur.SetEvolution(dir, true)
	if cur.Evolution.State != "first" {
		t.Errorf("plain run must not chain with a redacted one: %+v", cur.Evolution)
	}
	cur.Redact(salt)
	cur.SetEvolution(dir, true)
	if cur.Evolution.State != "compared" {
		t.Errorf("redacted run must chain with the earlier redacted one: %+v", cur.Evolution)
	}
}
