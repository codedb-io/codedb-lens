-- Runs in each database.
SELECT s.schemaname || '.' || s.relname AS "table",
       s.n_live_tup AS live_rows,
       s.n_dead_tup AS dead_rows,
       round(100.0 * s.n_dead_tup / nullif(s.n_live_tup + s.n_dead_tup, 0), 1) AS dead_pct,
       to_char(greatest(s.last_autovacuum, s.last_vacuum), 'YYYY-MM-DD HH24:MI') AS last_vacuum,
       to_char(greatest(s.last_autoanalyze, s.last_analyze), 'YYYY-MM-DD HH24:MI') AS last_analyze,
       s.autovacuum_count,
       CASE WHEN 100.0 * s.n_dead_tup / nullif(s.n_live_tup + s.n_dead_tup, 0) >= {{confirmed_dead_pct}}
            THEN 'confirmed' ELSE 'hint' END AS _confidence
FROM pg_stat_user_tables s
WHERE s.n_dead_tup >= {{min_dead}}
  AND 100.0 * s.n_dead_tup / nullif(s.n_live_tup + s.n_dead_tup, 0) >= {{dead_pct}}
ORDER BY s.n_dead_tup DESC
LIMIT 20;
