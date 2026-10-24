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
