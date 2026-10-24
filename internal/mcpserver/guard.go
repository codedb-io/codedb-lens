package mcpserver

import (
	"regexp"
	"strings"
)

// Layer 1 of query's read-only defence (plan §4): one SELECT/WITH statement, no write keyword, no function with a
// side effect. The rolled-back transaction (adapter.ReadOnly) and the login are what hold if this misses something.

// Tokens that hide text from the keyword scan, matched left to right like each server's own lexer: comments,
// strings, quoted identifiers. The dialects differ, and a mismatch is how a keyword hides, so each has its own.
var guardNoise = map[string]*regexp.Regexp{
	"mssql": regexp.MustCompile(`(?s)--[^\n]*|/\*.*?\*/|'[^']*'|"[^"]*"|\[[^\]]*\]`),
	// E'...' takes backslash escapes; [ ] are array subscripts, not identifiers.
	"pg": regexp.MustCompile(`(?s)--[^\n]*|/\*.*?\*/|(?i:\bE)'(?:[^'\\]|\\.|'')*'|'[^']*'|"[^"]*"`),
}

// Words outside comments and literals that refuse the query. SQL Server runs statements without ";", so every
// statement starter is here; the rest are side effects that a rollback does not undo, or locks.
var guardDeny = regexp.MustCompile(`(?i)(?:^|[^a-z_@#$])(` + strings.Join([]string{
	// statements
	`insert`, `update`, `delete`, `merge`, `alter`, `create`, `drop`, `truncate`, `grant`, `revoke`, `deny`,
	`exec`, `execute`, `dbcc`, `backup`, `restore`, `kill`, `shutdown`, `reconfigure`, `checkpoint`, `use`, `go`,
	`set`, `declare`, `begin`, `commit`, `rollback`, `save`, `savepoint`, `waitfor`, `enable`, `disable`, `receive`,
	`send`, `conversation`, `setuser`, `revert`, `writetext`, `updatetext`, `vacuum`, `analyze`, `reindex`,
	`cluster`, `copy`, `into`, `call`, `do`, `lock`, `listen`, `notify`, `unlisten`, `prepare`, `deallocate`,
	`refresh`, `discard`, `reset`, `load`, `import`,
	// SQL Server
	`xp_\w+`, `sp_\w+`, `openrowset`, `openquery`, `opendatasource`, `next\s+value\s+for`,
	`updlock`, `xlock`, `holdlock`, `tablock`, `tablockx`,
	// PostgreSQL
	`for\s+(?:key\s+)?share`, `pg_terminate_backend`, `pg_cancel_backend`, `set_config`, `nextval`, `setval`,
	`lo_\w+`, `dblink\w*`, `pg_read_file`, `pg_read_binary_file`, `pg_ls_\w+`, `pg_stat_file`, `pg_file_\w+`,
	`pg_reload_conf`, `pg_rotate_logfile`, `pg_sleep\w*`, `pg_advisory\w*`, `pg_try_advisory\w*`, `pg_notify`,
	`pg_switch_wal`, `pg_create_\w+`, `pg_drop_\w+`, `pg_replication_origin_\w+`, `pg_logical_\w+`, `pg_promote`,
	`pg_wal_replay_\w+`, `pg_stat_reset\w*`, `pg_stat_statements_reset`, `pg_\w*backup\w*`,
	`pg_import_system_collations`, `query_to_xml\w*`, `cursor_to_xml\w*`, `txid_current`,
}, "|") + `)\b`)

var (
	guardStart  = regexp.MustCompile(`(?i)^(select|with)\b`)
	dollarQuote = regexp.MustCompile(`\$\w*\$`)
)

// Write mode (Q46). Refused even when the user turned writing on. The list reads the raw text, comments and
// strings included, so no lexer mismatch and no EXEC('...') can hide a word, at the price of refusing one that only
// appears in a comment or a literal. It is defence in depth: the user's confirmation and the login come first.
var writeDeny = map[string]*regexp.Regexp{
	"mssql": denyList(writeDenyCommon, []string{
		// transaction control: inside a preview it would commit what the user was told is rolled back
		`commit`, `rollback`, `begin\s+(?:tran|transaction|distributed)`, `save\s+(?:tran|transaction)`,
		// identity, trust and ownership
		`(?:exec|execute)\s+as`, `setuser`, `sp_setapprole`, `alter\s+authorization`, `trustworthy`, `db_chaining`,
		// code and commands outside the database: CLR, Agent jobs, linked servers, external scripts, files
		`(?:create|alter)\s+assembly`, `sp_\w*job\w*`, `cmdexec`, `sp_\w*linkedserver\w*`, `sp_addextendedproc`,
		`sp_serveroption`, `external`, `bulk`, `on\s+all\s+server`, `(?:create|alter)\s+(?:endpoint|credential)`,
		// dynamic SQL in all its forms
		`sp_\w*exec\w*`, `sp_prep\w*`, `sp_cursor\w*`,
	}, []string{`(?:exec|execute)\s*[(@]`}),
	"pg": denyList(writeDenyCommon, []string{
		`commit`, `rollback`, `abort`, `savepoint`, `release\s+savepoint`, `start\s+transaction`, `prepare\s+transaction`,
		`set\s+(?:session\s+|local\s+)?role`, `session\s+authorization`, `owner\s+to`, `security\s+definer`,
		`(?:create|alter)\s+extension`, `plpython\w*`, `plperlu`, `pltclu`, `language\s+(?:c|internal)`,
		`foreign\s+data\s+wrapper`, `create\s+server`, `user\s+mapping`,
		`do`, `execute`, // anonymous blocks and dynamic SQL
	}, nil),
}

// writeDenyCommon: server-wide, security and file-system reach on both engines.
var writeDenyCommon = []string{
	`shutdown`, `drop\s+database`, `reconfigure`, `alter\s+system`, `alter\s+server`, `xp_\w+`, `sp_configure`, `sp_oa\w+`,
	`sp_add\w*(?:login|member)`, `sp_drop\w*(?:login|member)`, `(?:create|alter|drop)\s+(?:login|role|user)`,
	`grant`, `revoke`, `deny`, `openrowset`, `openquery`, `opendatasource`, `copy`, `program`, `lo_\w+`, `dblink\w*`,
	`pg_read_\w*file`, `pg_ls_\w+`, `pg_file_\w+`, `pg_reload_conf`, `load`,
}

// denyList matches any of words (whole words; "$" counts as a separator, so $$GRANT$$ is caught) or of raw
// (patterns that end in punctuation, where \b would not fit).
func denyList(common, words, raw []string) *regexp.Regexp {
	alts := append([]string{}, raw...)
	for _, w := range append(append([]string{}, common...), words...) {
		alts = append(alts, w+`\b`)
	}
	return regexp.MustCompile(`(?i)(?:^|[^a-z_@#])(` + strings.Join(alts, "|") + `)`)
}

var leadingComments = regexp.MustCompile(`(?s)^(?:\s*(?:--[^\n]*|/\*.*?\*/))*`)

// Statements a transaction cannot hold: no preview, so they need dry_run false.
var writeDirect = map[string]*regexp.Regexp{
	"mssql": regexp.MustCompile(`(?i)^\s*(alter\s+database|create\s+database|backup|restore|dbcc)\b`),
	"pg":    regexp.MustCompile(`(?i)^\s*(vacuum|alter\s+database|create\s+database|(create|drop|reindex)\b.*\bconcurrently|cluster\s*;?\s*$)`),
}

// writeGuard says whether sql may run through execute_sql, and whether it must run without a transaction.
func writeGuard(engine, sql string) (word string, direct, ok bool) {
	deny := writeDeny[engine]
	if deny == nil || strings.TrimSpace(sql) == "" {
		return "", false, false
	}
	if m := deny.FindStringSubmatch(sql); m != nil {
		return strings.ToUpper(strings.Join(strings.Fields(strings.TrimRight(m[1], "(@")), " ")), false, false
	}
	// A leading comment cannot hide a Direct statement from this check; one that slips through runs in a
	// transaction and the server refuses it ("cannot run inside a transaction block"): the safe way to fail.
	return "", writeDirect[engine].MatchString(leadingComments.ReplaceAllString(sql, "")), true
}

// guard says whether sql may run as query on engine. A refusal names the word, or none when the shape is wrong
// (more than one statement, not a SELECT/WITH, a nested comment or a PostgreSQL dollar quote).
func guard(engine, sql string) (word string, ok bool) {
	noise := guardNoise[engine]
	if noise == nil {
		return "", false
	}
	for _, m := range noise.FindAllString(sql, -1) {
		if strings.HasPrefix(m, "/*") && strings.Contains(m[2:], "/*") {
			return "", false // both servers nest block comments; this regex does not, so it could end one early
		}
	}
	code := strings.TrimSpace(noise.ReplaceAllString(sql, " "))
	code = strings.TrimSpace(strings.TrimSuffix(code, ";"))
	if strings.Contains(code, ";") || !guardStart.MatchString(code) || engine == "pg" && dollarQuote.MatchString(code) {
		return "", false
	}
	if m := guardDeny.FindStringSubmatch(code); m != nil {
		return strings.ToUpper(m[1]), false
	}
	return "", true
}
