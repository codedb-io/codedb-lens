# MSSQL-VERSION-001 · Version out of support

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed (past end of support) · Hint (within 365 days) | `public` | SQL Server 2012–2022, RDS (not Azure SQL Database / MI, which are versionless) |

## What it checks

`SERVERPROPERTY('ProductMajorVersion')` against the Microsoft extended-support end date bundled in Lens:

| Version | Extended support ends |
|---|---|
| SQL Server 2014 | 2024-07-09 |
| SQL Server 2016 | 2026-07-14 |
| SQL Server 2017 | 2027-10-12 |
| SQL Server 2019 | 2030-01-08 |
| SQL Server 2022 | 2033-01-11 |

## Why it matters

Out-of-support versions get no security fixes. A major upgrade takes months of planning and testing.

## Next action

Plan the upgrade to a supported version: check application compatibility and each database's compatibility level.

## Limitations

- Dates come from the table in this Lens version; update Lens to refresh them.
- Cumulative-update (CU) currency is not checked yet.
