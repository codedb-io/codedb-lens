-- Community EOL dates. Source: postgresql.org/support/versioning
WITH lifecycle (major, eol) AS (
    VALUES (11, date '2023-11-09'),
           (12, date '2024-11-21'),
           (13, date '2025-11-13'),
           (14, date '2026-11-12'),
           (15, date '2027-11-11'),
           (16, date '2028-11-09'),
           (17, date '2029-11-08'),
           (18, date '2030-11-14')
)
SELECT current_setting('server_version') AS version,
       l.eol AS end_of_life,
       l.eol - current_date AS days_left,
       CASE WHEN l.eol < current_date THEN 'confirmed' ELSE 'hint' END AS _confidence
FROM lifecycle l
WHERE l.major = current_setting('server_version_num')::int / 10000
  AND l.eol < current_date + {{warn_days}}::int;
