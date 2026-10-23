# MSSQL-TEMPDB-001 · TempDB files not laid out for concurrency

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Confirmed | `VIEW ANY DEFINITION` + `VIEW SERVER STATE` | SQL Server 2017+, Azure SQL MI, RDS |

## What it checks

From `sys.master_files` (database 2) and `sys.dm_os_sys_info`:

- Fewer data files than logical CPUs, up to 8.
- Data files with different sizes (SQL Server fills them proportionally, so the biggest one gets most of the load).
- Any TempDB file growing in percent.

## Why it matters

TempDB is shared by the whole instance. Too few or uneven files concentrate allocation on one file and cause
`PAGELATCH_*` waits on allocation pages; percent growth makes every growth event larger and slower.

## Next action

```sql
ALTER DATABASE tempdb MODIFY FILE (NAME = tempdev, SIZE = 8GB, FILEGROWTH = 512MB);
ALTER DATABASE tempdb ADD FILE (NAME = temp2, FILENAME = '<path>\tempdb2.ndf', SIZE = 8GB, FILEGROWTH = 512MB);
```

Size changes that shrink a file take effect after a restart.

## How to verify

List the files with `SELECT name, size * 8 / 1024 AS size_mb, growth, is_percent_growth FROM tempdb.sys.database_files`: data files must have the same size, fixed growth in MB (`is_percent_growth = 0`) and one file per logical CPU up to 8. New files and growth changes apply right away; shrinking or resizing existing files may need a restart. Then watch for PAGELATCH waits on tempdb.

## Limitations

Eight files is a starting point; heavy contention may need more, in multiples of 4.
