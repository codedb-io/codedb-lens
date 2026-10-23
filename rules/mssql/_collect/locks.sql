-- Same chain as MSSQL-BLOCKING-001, from 1 s up: the series rules decide what is long.
SELECT /* codeDB Lens */ r.session_id                                    AS blocked_session,
       r.blocking_session_id                           AS blocked_by,
       DB_NAME(r.database_id)                          AS [database],
       CAST(r.wait_time / 1000.0 AS decimal(12, 1))    AS waiting_s,
       r.wait_type,
       r.wait_resource,
       bs.status                                       AS blocker_status,
       bs.login_name                                   AS blocker_login,
       bs.host_name                                    AS blocker_host,
       bs.program_name                                 AS blocker_program,
       bs.open_transaction_count                       AS blocker_open_trans,
       CONVERT(varchar(18), r.query_hash, 1)           AS query_hash,
       SUBSTRING(t.text, r.statement_start_offset / 2 + 1,
           (CASE r.statement_end_offset WHEN -1 THEN DATALENGTH(t.text) ELSE r.statement_end_offset END
            - r.statement_start_offset) / 2 + 1)       AS query
FROM sys.dm_exec_requests r
JOIN sys.dm_exec_sessions bs ON bs.session_id = r.blocking_session_id
OUTER APPLY sys.dm_exec_sql_text(r.sql_handle) t
WHERE r.blocking_session_id <> 0
  AND r.session_id <> @@SPID
  AND r.wait_time >= 1000
ORDER BY r.wait_time DESC;
