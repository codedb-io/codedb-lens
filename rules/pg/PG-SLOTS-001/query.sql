WITH s AS (
    SELECT slot_name, slot_type, active, wal_status,
           round(pg_wal_lsn_diff(pg_current_wal_lsn(), coalesce(restart_lsn, pg_current_wal_lsn())) / 1048576.0, 1) AS retained_mb
    FROM pg_replication_slots
    WHERE NOT pg_is_in_recovery()
)
SELECT 'slot' AS kind,
       s.slot_name AS name,
       s.slot_type AS type,
       s.active,
       s.retained_mb,
       s.wal_status,
       NULL::numeric AS replay_lag_s,
       CASE WHEN s.wal_status IN ('lost', 'unreserved') OR s.retained_mb >= {{confirmed_retained_mb}}
            THEN 'confirmed' ELSE 'hint' END AS _confidence
FROM s
WHERE (NOT s.active AND s.retained_mb >= {{retained_mb}})
   OR s.wal_status IN ('lost', 'unreserved')
UNION ALL
SELECT 'replica', coalesce(nullif(r.application_name, ''), r.client_addr::text), 'physical', true,
       round(pg_wal_lsn_diff(r.sent_lsn, r.replay_lsn) / 1048576.0, 1), NULL,
       round(extract(epoch FROM r.replay_lag)::numeric, 1), 'hint'
FROM pg_stat_replication r
WHERE extract(epoch FROM r.replay_lag) >= {{lag_s}};
