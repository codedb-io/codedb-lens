# PG-VERSION-001 · Version out of support

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| High | Confirmed (past EOL) · Hint (within 365 days) | `public` | All PostgreSQL versions and services |

## What it checks

`server_version_num` against the community end-of-life dates bundled in Lens (14: 2026-11-12, 15: 2027-11-11,
16: 2028-11-09, 17: 2029-11-08, 18: 2030-11-14).

## Why it matters

Out-of-support versions get no security or bug fixes. Major upgrades need planning and testing.

## Next action

Plan a major upgrade: `pg_upgrade`, logical replication, or your cloud provider's upgrade path.

## Limitations

- Cloud providers may offer paid extended support beyond the community date.
- Minor-version currency is not checked yet.
