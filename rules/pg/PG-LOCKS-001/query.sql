SELECT a.pid AS blocked_pid,
       pg_blocking_pids(a.pid)::text AS blocked_by,
       a.datname AS database,
       round(extract(epoch FROM now() - a.query_start)::numeric, 0) AS waiting_s,
       a.wait_event,
       b.pid AS blocker_pid,
       b.state AS blocker_state,
       b.usename AS blocker_user,
       b.application_name AS blocker_application,
       round(extract(epoch FROM now() - b.xact_start)::numeric, 0) AS blocker_transaction_age_s,
       left(regexp_replace(a.query, '\s+', ' ', 'g'), 200) AS blocked_query
FROM pg_stat_activity a
JOIN pg_stat_activity b ON b.pid = (pg_blocking_pids(a.pid))[1]
WHERE a.wait_event_type = 'Lock'
  AND a.pid <> pg_backend_pid()
  AND now() - a.query_start >= {{min_wait_s}} * interval '1 second'
ORDER BY a.query_start;
