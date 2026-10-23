-- Latest minor per supported major, as of 2026-10-22; update every release (see RELEASING.md).
-- Source: postgresql.org/support/versioning and the release notes at postgresql.org/docs/release/
WITH latest (major, minor, released) AS (
    VALUES (14, 24, date '2026-08-13'),
           (15, 19, date '2026-08-13'),
           (16, 15, date '2026-08-13'),
           (17, 11, date '2026-08-13'),
           (18, 6, date '2026-08-13')
),
cur AS (
    SELECT current_setting('server_version_num')::int / 10000 AS major,
           current_setting('server_version_num')::int % 100 AS minor
)
SELECT current_setting('server_version') AS installed_version,
       l.major || '.' || l.minor AS latest_version,
       l.released AS latest_release_date,
       CASE WHEN c.minor > l.minor THEN NULL ELSE l.minor - c.minor END AS minors_behind,
       CASE WHEN c.minor > l.minor THEN 'hint' ELSE 'confirmed' END AS _confidence
FROM cur c
JOIN latest l ON l.major = c.major
WHERE c.minor <> l.minor;
