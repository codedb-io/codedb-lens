SELECT /* codeDB Lens */ r.session_id,
       NULLIF(r.blocking_session_id, 0)                      AS blocked_by,
       DB_NAME(r.database_id)                                AS [database],
       r.status,
       r.command,
       r.wait_type,
       r.wait_time                                           AS wait_ms,
       CAST(r.total_elapsed_time / 1000.0 AS decimal(12, 1)) AS elapsed_s,
       r.cpu_time                                            AS cpu_ms,
       r.logical_reads,
       s.login_name,
       s.host_name,
       s.program_name,
       CONVERT(varchar(18), r.query_hash, 1)                 AS query_hash,
       SUBSTRING(t.text, r.statement_start_offset / 2 + 1,
           (CASE r.statement_end_offset WHEN -1 THEN DATALENGTH(t.text) ELSE r.statement_end_offset END
            - r.statement_start_offset) / 2 + 1)             AS query
FROM sys.dm_exec_requests r
JOIN sys.dm_exec_sessions s ON s.session_id = r.session_id
OUTER APPLY sys.dm_exec_sql_text(r.sql_handle) t
WHERE s.is_user_process = 1
  AND r.session_id <> @@SPID
ORDER BY r.total_elapsed_time DESC;
