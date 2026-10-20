# PG-BLOAT-001 · Estimated table bloat

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Hint (always an estimate) | `pg_monitor` + `CONNECT` on each database + `SELECT` on the tables (or `pg_read_all_data`) | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

Runs **in each database**, for tables of at least 100 MB (`min_size_mb`). It estimates how many pages the table *should*
need from its row count and average row width (`pg_stats`, respecting `fillfactor`) and compares with the pages it uses.
Reported at 40% or more estimated waste (`bloat_pct`).

## Statistics and permissions

`pg_stats` only shows columns the login can `SELECT`. Lens won't guess:

- Tables never analyzed are skipped.
- If large tables have statistics this login can't see, the rule reports **No data** with the count, instead of a false OK.
  Grant `SELECT` on those tables, or `pg_read_all_data` (PostgreSQL 14+), if you want this rule.

## Why it matters

A bloated table costs more disk, more cache and more I/O for the same rows. Regular `VACUUM` makes the space reusable
inside the table but doesn't return it.

## Next action

Confirm with `pgstattuple_approx('<table>')` first. Reclaim space online with `pg_repack` or `pg_squeeze`; `VACUUM FULL`
holds an exclusive lock for the whole run. Then fix the cause (`PG-AUTOVAC-001`, `PG-LONGTX-001`).

## Limitations

Heap only (indexes not estimated). The estimate assumes no row-level alignment surprises and current statistics; run
`ANALYZE` if they are old.
