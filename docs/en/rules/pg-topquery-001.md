# PG-TOPQUERY-001 · A single query dominates execution time

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Hint | `pg_monitor` (`pg_read_all_stats`) + `pg_stat_statements` | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

Top-level statements in `pg_stat_statements`. Reported when the most expensive query takes at least 40% of the total
execution time (`dominant_pct`) and the total is at least 60 s (`min_total_s`). The evidence lists the top 5 with share,
calls, mean time, rows per call and cache hit ratio.

Without the extension the rule reports *No data* and points to `PG-PGSS-001`.

## Why it matters

When one query dominates, fixing it is the change with the highest return.

## Next action

```sql
EXPLAIN (ANALYZE, BUFFERS) <query with real parameters>;
```

Look for sequential scans on large tables, row estimates far from reality, and sorts or hashes spilling to disk. Also
check whether it runs more often than the application needs (a low *rows per call* with huge *calls* suggests an N+1 pattern).

## Limitations

Totals accumulate since the last `pg_stat_statements_reset()` or restart, so you can't tell when a query started dominating.
Without `pg_read_all_stats`, other users' query texts show as `<insufficient privilege>`.
