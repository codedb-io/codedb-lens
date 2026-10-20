-- Runs in each database.
WITH i AS (
    SELECT x.indexrelid, x.indrelid, x.indisunique, x.indisprimary, x.indisvalid,
           x.indkey::text AS keys, x.indclass::text AS opclasses,
           coalesce(pg_get_expr(x.indexprs, x.indrelid), '') AS exprs,
           coalesce(pg_get_expr(x.indpred, x.indrelid), '') AS pred,
           s.idx_scan,
           pg_relation_size(x.indexrelid) AS bytes
    FROM pg_index x
    JOIN pg_stat_user_indexes s ON s.indexrelid = x.indexrelid
)
SELECT i.indrelid::regclass::text AS "table", i.indexrelid::regclass::text AS index, 'invalid' AS issue,
       i.idx_scan AS scans, round(i.bytes / 1048576.0, 1) AS size_mb, NULL::text AS duplicate_of, 'confirmed' AS _confidence
FROM i WHERE NOT i.indisvalid
UNION ALL
SELECT i.indrelid::regclass::text, i.indexrelid::regclass::text, 'unused',
       i.idx_scan, round(i.bytes / 1048576.0, 1), NULL, 'hint'
FROM i
WHERE i.indisvalid AND i.idx_scan = 0 AND NOT i.indisunique AND NOT i.indisprimary
  AND i.bytes >= {{min_unused_mb}} * 1048576.0
UNION ALL
SELECT i.indrelid::regclass::text, i.indexrelid::regclass::text, 'duplicate',
       i.idx_scan, round(i.bytes / 1048576.0, 1), d.indexrelid::regclass::text, 'confirmed'
FROM i
JOIN i AS d ON d.indrelid = i.indrelid AND d.indexrelid < i.indexrelid
           AND d.keys = i.keys AND d.opclasses = i.opclasses AND d.exprs = i.exprs AND d.pred = i.pred
WHERE NOT i.indisprimary
ORDER BY 1, 2;
