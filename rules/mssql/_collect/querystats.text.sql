SELECT /* codeDB Lens */ CONVERT(varchar(18), qs.query_hash, 1) AS hash,
       MAX(SUBSTRING(t.text, qs.statement_start_offset / 2 + 1,
           (CASE qs.statement_end_offset WHEN -1 THEN DATALENGTH(t.text) ELSE qs.statement_end_offset END
            - qs.statement_start_offset) / 2 + 1)) AS text
FROM sys.dm_exec_query_stats qs
OUTER APPLY sys.dm_exec_sql_text(qs.sql_handle) t
WHERE qs.query_hash IN ({{hashes}})
GROUP BY qs.query_hash;
