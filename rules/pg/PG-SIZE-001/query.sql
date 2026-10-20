-- Runs in each database.
SELECT n.nspname || '.' || c.relname AS relation,
       round(pg_total_relation_size(c.oid) / 1073741824.0, 2) AS total_gb,
       round(pg_relation_size(c.oid) / 1073741824.0, 2) AS table_gb,
       round(pg_indexes_size(c.oid) / 1073741824.0, 2) AS indexes_gb,
       c.reltuples::bigint AS est_rows,
       round(100.0 * pg_total_relation_size(c.oid) / nullif(pg_database_size(current_database()), 0), 1) AS pct_of_database
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'm')
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND n.nspname NOT LIKE 'pg_toast%'
  AND pg_total_relation_size(c.oid) >= {{min_rel_gb}} * 1073741824.0
ORDER BY pg_total_relation_size(c.oid) DESC
LIMIT 10;
