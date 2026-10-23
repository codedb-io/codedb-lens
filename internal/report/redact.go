package report

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Evidence columns by what they hold. Anything not listed (numbers, dates, statuses, wait types ...) is left alone.
// ponytail: a custom rule (--rules-dir) with another sensitive column name is not masked; add it here.
var maskKind = map[string]string{
	"database": "db", "table": "obj", "index": "obj", "relation": "obj", "keys": "obj", "duplicate_of": "obj",
	"equality_columns": "obj", "inequality_columns": "obj", "included_columns": "obj",
	"job": "job", "file": "path",
	"user": "login", "blocker_user": "login", "blocker_login": "login",
	"application_name": "app", "blocker_application": "app", "blocker_program": "app",
	"client_addr": "addr", "blocker_host": "addr",
	// record collectors (series rule evidence)
	"login_name": "login", "user_name": "login", "host_name": "addr", "program_name": "app", "wait_resource": "obj",
}

// Columns whose whole value goes: query text and the Agent step message can carry literals and names.
var dropCol = map[string]bool{"blocked_query": true, "query": true, "last_query": true, "message": true}

// Generic "name" column: what it holds depends on the row's "kind" (MSSQL-SPACE, PG-SLOTS).
var nameKind = map[string]string{"volume": "path", "log": "db", "slot": "slot", "replica": "addr"}

var systemDB = map[string]bool{"master": true, "tempdb": true, "msdb": true, "model": true, "mssqlsystemresource": true,
	"postgres": true, "template0": true, "template1": true}

const redactedText = "[redacted]"

// Redact masks what identifies the customer's environment, in place: host, database and object names, logins,
// applications, addresses, jobs, slots, volumes and files become <kind>-<8 hex> (HMAC-SHA256 with salt, so the
// same name gets the same token on this machine); query text is dropped. Numbers, dates and statuses stay.
func (r *Report) Redact(salt []byte) {
	seen := map[string]string{} // original -> token, to scrub the free text in Reason
	mask := func(kind, v string) string {
		if kind == "db" && systemDB[strings.ToLower(v)] || v == "" {
			return v
		}
		h := hmac.New(sha256.New, salt)
		h.Write([]byte(kind + "\x00" + strings.ToLower(v))) // case-insensitive: hosts and Windows names differ in case only
		t := kind + "-" + hex.EncodeToString(h.Sum(nil)[:4])
		seen[v] = t
		return t
	}
	r.Server.Host = mask("host", r.Server.Host)
	for i := range r.Results {
		res := &r.Results[i]
		kindAt := -1
		for j, c := range res.Columns {
			if c == "kind" {
				kindAt = j
			}
		}
		for _, row := range res.Rows {
			for j, v := range row {
				if v == nil || j >= len(res.Columns) {
					continue
				}
				col, _, _ := strings.Cut(res.Columns[j], "@")
				s := fmt.Sprint(v)
				if b, ok := v.([]byte); ok {
					s = string(b)
				}
				switch kind := maskKind[col]; {
				case dropCol[col]:
					if s != "" {
						row[j] = redactedText
					}
				case col == "name" && kindAt >= 0:
					row[j] = mask(firstNonEmpty(nameKind[fmt.Sprint(row[kindAt])], "obj"), s)
				case kind != "":
					row[j] = mask(kind, s)
				}
			}
		}
		if list, ok := strings.CutPrefix(res.Reason, "skipped databases: "); ok { // "db (no access); db (error)": names never seen in rows
			for _, e := range strings.Split(list, "; ") {
				name, _, _ := strings.Cut(e, " (")
				mask("db", name)
			}
		}
	}
	var names []string
	for v := range seen {
		if len(v) >= 2 { // ponytail: 1-character names would garble the text; accepted leak
			names = append(names, v)
		}
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) }) // longest first
	var pairs []string
	for _, v := range names {
		pairs = append(pairs, v, seen[v])
	}
	scrub := strings.NewReplacer(pairs...)
	for i := range r.Results {
		r.Results[i].Reason = scrub.Replace(r.Results[i].Reason)
	}
	r.Redacted = true
}
