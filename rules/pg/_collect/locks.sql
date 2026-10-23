-- Same chain as PG-LOCKS-001, from 1 s up: the series rules decide what is long.
SELECT a.pid                                                         AS blocked_pid,
       pg_blocking_pids(a.pid)::text                                 AS blocked_by,
       a.datname                                                     AS database,
       round(extract(epoch FROM now() - a.query_start)::numeric, 1)  AS waiting_s,
       a.wait_event,
       b.pid                                                         AS blocker_pid,
       b.state                                                       AS blocker_state,
       b.usename                                                     AS blocker_user,
       b.application_name                                            AS blocker_application,
       round(extract(epoch FROM now() - b.xact_start)::numeric, 1)   AS blocker_transaction_s,
       coalesce(a.query_id::text, 'md5:' || md5(a.query))            AS query_hash,
       a.query
FROM pg_stat_activity a
JOIN pg_stat_activity b ON b.pid = (pg_blocking_pids(a.pid))[1]
WHERE a.wait_event_type = 'Lock'
  AND a.pid <> pg_backend_pid()
  AND now() - a.query_start >= interval '1 second'
ORDER BY a.query_start;
