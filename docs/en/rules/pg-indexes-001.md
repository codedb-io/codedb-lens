# PG-INDEXES-001 · Unused, duplicate or invalid indexes

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Low | Confirmed (invalid, duplicate) · Hint (unused) | `pg_monitor` + `CONNECT` on each database | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

Runs **in each database**, on `pg_index` and `pg_stat_user_indexes`:

- **Invalid:** `indisvalid = false`, usually left by a failed `CREATE INDEX CONCURRENTLY`.
- **Unused:** zero scans, not unique or primary key, at least 10 MB (`min_unused_mb`).
- **Duplicate:** same columns, operator classes, expressions and predicate as another index on the table (a primary key
  is never reported as the duplicate).

## Why it matters

Every index is written on every insert and on non-HOT updates, takes cache memory and backup space, and is rebuilt by
maintenance. Invalid indexes are maintained but never used for reads.

## Next action

```sql
DROP INDEX CONCURRENTLY <index>;   -- invalid or duplicate
```

For unused indexes, confirm over a full business cycle and check `pg_stat_user_indexes` on each replica before dropping.

## Limitations

Usage counters reset with `pg_stat_reset()`; replicas keep their own counters.
