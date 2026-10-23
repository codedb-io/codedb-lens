-- Usable slots: max_connections minus the ones reserved for superusers.
SELECT /* codeDB Lens */ count(*) FILTER (WHERE backend_type = 'client backend')                                  AS connections,
       count(*) FILTER (WHERE backend_type = 'client backend' AND state = 'active')             AS active,
       count(*) FILTER (WHERE backend_type = 'client backend' AND state = 'idle in transaction') AS idle_in_transaction,
       current_setting('max_connections')::int
         - current_setting('superuser_reserved_connections')::int                               AS max_connections
FROM pg_stat_activity;
