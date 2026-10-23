-- query_id needs compute_query_id (on by default with pg_stat_statements); otherwise the text's md5 is the hash.
SELECT /* codeDB Lens */ a.pid,
       a.datname                                                     AS database,
       a.usename                                                     AS user_name,
       a.application_name,
       a.client_addr::text                                           AS client_addr,
       a.state,
       a.wait_event_type,
       a.wait_event,
       round(extract(epoch FROM now() - a.query_start)::numeric, 1)  AS running_s,
       round(extract(epoch FROM now() - a.xact_start)::numeric, 1)   AS transaction_s,
       coalesce(a.query_id::text, 'md5:' || md5(a.query))            AS query_hash,
       a.query
FROM pg_stat_activity a
WHERE a.backend_type = 'client backend'
  AND a.state IS DISTINCT FROM 'idle'
  AND a.pid <> pg_backend_pid()
ORDER BY a.query_start;
