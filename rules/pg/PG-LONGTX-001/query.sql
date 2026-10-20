SELECT a.pid,
       a.datname AS database,
       a.usename AS "user",
       a.application_name,
       a.client_addr::text AS client_addr,
       a.state,
       round(extract(epoch FROM now() - a.xact_start)::numeric, 0) AS transaction_age_s,
       round(extract(epoch FROM now() - a.state_change)::numeric, 0) AS in_state_s,
       left(regexp_replace(a.query, '\s+', ' ', 'g'), 200) AS last_query
FROM pg_stat_activity a
WHERE a.backend_type = 'client backend'
  AND a.pid <> pg_backend_pid()
  AND a.xact_start IS NOT NULL
  AND (now() - a.xact_start > {{max_tx_s}} * interval '1 second'
       OR (a.state LIKE 'idle in transaction%' AND now() - a.state_change > {{idle_tx_s}} * interval '1 second'))
ORDER BY a.xact_start;
