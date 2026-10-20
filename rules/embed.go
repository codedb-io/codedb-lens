// Package rules embeds the rule catalog. Rules are data: rule.yaml + query.sql per folder.
package rules

import "embed"

//go:embed scoring.yaml mssql pg
var FS embed.FS
