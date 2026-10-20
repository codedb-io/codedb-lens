# PG-SIZE-001 · Large tables that dominate storage

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Low | Hint | `pg_monitor` + `CONNECT` on each database | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## What it checks

Runs **in each database** the login can connect to. Lists up to 10 tables and materialized views of at least 10 GB
(`min_rel_gb`) including indexes and TOAST, with their share of the database. Databases without `CONNECT` are listed as skipped.

## Why it matters

A handful of big tables usually drives disk growth, backup duration and vacuum time. They are where archiving,
partitioning and index cleanup pay off.

## Next action

For each table: can old rows be archived or partitioned by date? Are all its indexes used (`PG-INDEXES-001`)? Is it bloated
(`PG-BLOAT-001`)?

## Limitations

Shows current size, not growth: projecting when a disk fills needs history. Free disk space isn't visible through SQL.
