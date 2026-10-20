-- Runs in each database. Estimate: expected pages from row count and average row width (pg_stats)
-- versus actual pages. Heap only; fillfactor respected; 8-byte alignment, 24-byte tuple header, 4-byte line pointer.
WITH t AS (
    SELECT c.oid, n.nspname || '.' || c.relname AS "table", c.relpages, c.reltuples,
           coalesce((SELECT substring(o FROM 'fillfactor=(\d+)')::int FROM unnest(c.reloptions) o WHERE o LIKE 'fillfactor=%'), 100) AS ff,
           (SELECT sum((1 - s.null_frac) * s.avg_width) FROM pg_stats s WHERE s.schemaname = n.nspname AND s.tablename = c.relname) AS data_width
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE c.relkind = 'r'
      AND n.nspname NOT IN ('pg_catalog', 'information_schema')
      AND c.relpages::numeric * current_setting('block_size')::int >= {{min_size_mb}} * 1048576.0
),
e AS (
    SELECT t.*,
           ceil(t.reltuples::numeric / greatest(floor((current_setting('block_size')::int - 24) * t.ff / 100.0
                / (24 + ceil(t.data_width::numeric / 8) * 8 + 4)), 1)) AS expected_pages
    FROM t WHERE t.data_width IS NOT NULL AND t.reltuples > 0
),
-- No stats + no SELECT = hidden by permissions (refuse). No stats + SELECT = never analyzed (skip that table).
chk AS (SELECT count(*) FILTER (WHERE data_width IS NULL AND NOT has_table_privilege(oid, 'SELECT')) AS no_stats FROM t)
SELECT e."table",
       round(e.relpages::numeric * current_setting('block_size')::int / 1048576, 1) AS size_mb,
       round(100 * (e.relpages - e.expected_pages) / e.relpages::numeric, 1) AS est_bloat_pct,
       round((e.relpages - e.expected_pages) * current_setting('block_size')::int / 1048576.0, 1) AS est_wasted_mb,
       e.reltuples::bigint AS est_rows
FROM e CROSS JOIN chk
WHERE chk.no_stats = 0
  AND 100 * (e.relpages - e.expected_pages) / e.relpages::numeric >= {{bloat_pct}}
UNION ALL
-- Refuse to report a clean result when large tables have no visible statistics: surfaces as No data with this message.
SELECT NULL, NULL, NULL, NULL,
       cast(chk.no_stats || ' large table(s) whose column statistics this login cannot see (pg_stats needs SELECT on the table)' AS bigint)
FROM chk WHERE chk.no_stats > 0
ORDER BY 3 DESC NULLS LAST;
