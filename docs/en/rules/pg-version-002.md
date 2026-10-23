# PG-VERSION-002 · Minor version behind the latest release

| Severity | Confidence | Permission | Applies to |
|---|---|---|---|
| Medium | Confirmed (minor behind) · Hint (minor newer than the bundled table) | `public` | PostgreSQL 14–18, all services |

## What it checks

The installed minor (`server_version_num % 100`) against the latest minor of the same major bundled in Lens (14.24,
15.19, 16.15, 17.11, 18.6, all released 2026-08-13). It shows the installed version, the latest version and its release
date, and how many minors you are behind.

## Why it matters

Minor releases fix security issues, data corruption and frequent bugs, and are drop-in updates within the same major.
Running behind leaves known issues open.

## Next action

Update to the latest minor release: a binary swap and restart, with no dump or `pg_upgrade`. Read the release notes for
the minors you skip.

## How to verify

After the restart, run `SHOW server_version` and confirm the new minor is the target. Check the server log for startup errors, run `ALTER EXTENSION name UPDATE` if the release notes ask for it, and confirm replicas were updated too. Re-run codeDB Lens: the rule should be OK.

## Limitations

- The table comes from this Lens version; update Lens to refresh it.
- A minor newer than the table is reported as a *Hint* (the table may be outdated), never as OK.
- Managed services often lag behind community releases; if the provider has no newer minor, there is nothing to apply yet.
- Majors outside the table (13 and older, or not yet listed) are not checked; [PG-VERSION-001](pg-version-001.md) reports end of support.
