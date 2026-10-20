-- Runs in each database. Nonclustered, non-constraint indexes only.
WITH ix AS (
    SELECT i.object_id, i.index_id, i.name, i.filter_definition,
           OBJECT_SCHEMA_NAME(i.object_id) + '.' + OBJECT_NAME(i.object_id) AS [table],
           (SELECT STRING_AGG(CAST(c.name AS nvarchar(max)) + CASE WHEN ic.is_descending_key = 1 THEN ' DESC' ELSE '' END, ', ')
                   WITHIN GROUP (ORDER BY ic.key_ordinal)
            FROM sys.index_columns ic JOIN sys.columns c ON c.object_id = ic.object_id AND c.column_id = ic.column_id
            WHERE ic.object_id = i.object_id AND ic.index_id = i.index_id AND ic.key_ordinal > 0) AS keys
    FROM sys.indexes i
    WHERE i.type = 2 AND i.is_primary_key = 0 AND i.is_unique_constraint = 0 AND i.is_hypothetical = 0
      AND OBJECTPROPERTY(i.object_id, 'IsUserTable') = 1
),
u AS (
    SELECT ix.*,
           ISNULL(us.user_seeks + us.user_scans + us.user_lookups, 0) AS reads,
           ISNULL(us.user_updates, 0) AS writes,
           (SELECT SUM(ps.used_page_count) * 8 / 1024 FROM sys.dm_db_partition_stats ps
            WHERE ps.object_id = ix.object_id AND ps.index_id = ix.index_id) AS size_mb
    FROM ix
    LEFT JOIN sys.dm_db_index_usage_stats us
           ON us.database_id = DB_ID() AND us.object_id = ix.object_id AND us.index_id = ix.index_id
)
SELECT u.[table], u.name AS [index], 'unused' AS issue, u.keys, u.reads, u.writes, u.size_mb, CAST(NULL AS sysname) AS duplicate_of
FROM u
WHERE u.reads = 0 AND u.writes >= {{min_writes}}
UNION ALL
SELECT u.[table], u.name, 'duplicate', u.keys, u.reads, u.writes, u.size_mb, d.name
FROM u
JOIN u AS d ON d.object_id = u.object_id AND d.index_id < u.index_id
            AND d.keys = u.keys AND ISNULL(d.filter_definition, '') = ISNULL(u.filter_definition, '')
ORDER BY [table], [index];
