// Package rules embeds the rule catalog (rule.yaml + query.sql per folder) and the record collectors (_collect).
// Folders starting with "_" must be named explicitly: go:embed skips them inside a directory.
package rules

import "embed"

//go:embed scoring.yaml mssql pg mssql/_collect pg/_collect
var FS embed.FS
