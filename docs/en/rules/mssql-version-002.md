# MSSQL-VERSION-002 · Build behind the latest CU

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Confirmed (CU older than 180 days) · Hint (build newer than the bundled table) | `public` | SQL Server 2017–2025, RDS (not Azure SQL Database / MI, which are versionless) |

## What it checks

The installed build (`SERVERPROPERTY('ProductVersion')`) against the Microsoft version history bundled in Lens (RTM and
every cumulative update of 2017, 2019, 2022 and 2025). A build between two CUs, such as a security-only GDR build,
counts as the CU below it. The finding appears when the CU you run was released more than 180 days before the latest CU
of your version (`max_age_days`). It shows the installed build, the matching CU, the latest CU and its date, and the
days behind.

## Why it matters

Cumulative updates carry the bug and security fixes for your version. A build many months behind keeps known issues
open and makes the next patch a bigger jump.

## Next action

Test the latest cumulative update in a non-production copy and schedule it in a maintenance window.

## How to verify

After patching, run `SELECT SERVERPROPERTY('ProductVersion'), SERVERPROPERTY('ProductUpdateLevel')` and confirm the build matches the target CU or GDR. Check the SQL Server error log for startup errors and that jobs and replication or AG members came back healthy. Re-run codeDB Lens: the rule should be OK.

## Limitations

- The build table comes from this Lens version; update Lens to refresh it.
- A build newer than the table is reported as a *Hint* (the table may be outdated), never as OK.
- Versions older than 2017 are not covered; [MSSQL-VERSION-001](mssql-version-001.md) reports their end of support.
