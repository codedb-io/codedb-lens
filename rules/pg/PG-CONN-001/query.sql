WITH a AS (
    SELECT count(*) AS total, count(*) FILTER (WHERE state = 'idle') AS idle
    FROM pg_stat_activity
    WHERE backend_type = 'client backend'
),
m AS (SELECT current_setting('max_connections')::int AS max_conn)
SELECT 'connections near max_connections' AS issue,
       a.total AS connections,
       m.max_conn AS max_connections,
       round(100.0 * a.total / m.max_conn, 1) AS pct,
       CASE WHEN 100.0 * a.total / m.max_conn >= {{confirmed_pct}} THEN 'confirmed' ELSE 'hint' END AS _confidence
FROM a, m
WHERE 100.0 * a.total / m.max_conn >= {{hint_pct}}
UNION ALL
SELECT 'many idle connections (no pooler?)', a.idle, m.max_conn,
       round(100.0 * a.idle / nullif(a.total, 0), 1), 'hint'
FROM a, m
WHERE a.idle >= {{idle_min}}
  AND 100.0 * a.idle / nullif(a.total, 0) >= {{idle_pct}};
