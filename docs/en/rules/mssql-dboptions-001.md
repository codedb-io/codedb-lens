# MSSQL-DBOPTIONS-001 · Databases with risky options

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Confirmed | `VIEW ANY DEFINITION` | SQL Server 2017+, Azure SQL MI, RDS (not Azure SQL Database) |

## What it checks

`sys.databases` for every online user database (`database_id > 4`, no snapshots). A finding is raised when
`AUTO_SHRINK` or `AUTO_CLOSE` is on, or when `PAGE_VERIFY` is anything other than `CHECKSUM`.

## Why it matters

- **AUTO_SHRINK** shrinks the files in the background, fragments indexes and then fights autogrowth, which grows them again.
- **AUTO_CLOSE** closes the database after the last connection leaves, so the next connection pays the cost of opening it.
- **PAGE_VERIFY other than CHECKSUM** (`TORN_PAGE_DETECTION` or `NONE`) lets damaged pages go unnoticed until something reads them, often after the damage reached the backups.

## Next action

```sql
ALTER DATABASE [db] SET AUTO_SHRINK OFF, AUTO_CLOSE OFF;
ALTER DATABASE [db] SET PAGE_VERIFY CHECKSUM;
```

Pages only get a checksum when they are next written, so take a full backup and run `DBCC CHECKDB` afterwards.

## How to verify

```sql
SELECT name, is_auto_shrink_on, is_auto_close_on, page_verify_option_desc
FROM sys.databases WHERE database_id > 4;
```

Both flags must be `0` and `page_verify_option_desc` must be `CHECKSUM`.

## Limitations

- System databases are not checked.
- Not applicable to Azure SQL Database, where the platform manages these options.
- Some vendor applications ask for `AUTO_CLOSE` on SQL Server Express; confirm with the vendor before changing it.
