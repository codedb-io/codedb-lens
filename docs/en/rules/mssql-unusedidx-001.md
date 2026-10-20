# MSSQL-UNUSEDIDX-001 · Unused or duplicate indexes

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Low | Hint | `VIEW SERVER STATE` + `VIEW ANY DEFINITION` + `CONNECT ANY DATABASE` | SQL Server 2017+, Azure SQL MI, RDS |

## What it checks

Runs **in each user database** the login can open. For nonclustered indexes that don't back a primary key or unique constraint:

- **Unused:** no seeks, scans or lookups since the restart, but at least 1,000 writes (`min_writes`), so it only costs.
- **Duplicate:** same key columns, in the same order, and the same filter as another index on the table.

Databases the login can't open are listed as skipped, and if none can be read the rule reports *No data*.

## Why it matters

Each index is maintained on every insert, update and delete, is backed up and rebuilt. An index nobody reads is pure cost.

## Next action

Watch it through a full business cycle (month-end closing, quarterly reports) before acting. Script the index, then
disable it (`ALTER INDEX … DISABLE`) for a while before dropping it.

## Limitations

- Usage resets on restart. Check the *Started at* card in the report: a few days of uptime is not enough.
- Readable secondaries and replicas keep their own usage; an index unused here may be used there.
