# MSSQL-MISSINGIDX-001 · High-impact missing indexes

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Hint (always) | `VIEW SERVER STATE` | SQL Server 2017+, Azure SQL MI, RDS, Azure SQL Database |

## What it checks

The missing-index DMVs (`sys.dm_db_missing_index_*`) for user databases. A suggestion is reported when the optimizer
estimates at least 50% improvement (`min_impact_pct`) and its weighted benefit
`avg_total_user_cost × avg_user_impact × (seeks + scans)` is at least 100,000 (`min_improvement`). Top 20.

On Azure SQL Database only the connected database is checked.

## Why it matters

Each row is a query the optimizer compiled while wishing an index existed. Those queries read more pages and hold locks longer.

## Next action

Don't create the suggestions as they are. For each table: compare with the existing indexes, merge overlapping
suggestions into one, put equality columns first, and test on a copy before production.

## Limitations

- Always a hint: the DMV doesn't look at similar existing indexes and often suggests overlapping ones.
- Cleared on restart and when any index is created or dropped on the table.
- Queries with trivial plans never produce suggestions.
