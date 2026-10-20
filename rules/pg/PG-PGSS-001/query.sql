WITH s AS (
    SELECT current_setting('shared_preload_libraries') AS preload,
           position('pg_stat_statements' IN current_setting('shared_preload_libraries')) > 0 AS loaded,
           EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements') AS created
)
SELECT CASE WHEN NOT s.loaded THEN 'not in shared_preload_libraries' ELSE 'extension not created in this database' END AS problem,
       s.preload AS shared_preload_libraries,
       s.created AS extension_created,
       current_database() AS database
FROM s
WHERE NOT (s.loaded AND s.created);
