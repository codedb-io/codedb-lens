-- Microsoft lifecycle (extended support end). Source: learn.microsoft.com/lifecycle/products
WITH lifecycle (major, product, extended_end) AS (
    SELECT * FROM (VALUES
        (11, 'SQL Server 2012', CAST('2022-07-12' AS date)),
        (12, 'SQL Server 2014', CAST('2024-07-09' AS date)),
        (13, 'SQL Server 2016', CAST('2026-07-14' AS date)),
        (14, 'SQL Server 2017', CAST('2027-10-12' AS date)),
        (15, 'SQL Server 2019', CAST('2030-01-08' AS date)),
        (16, 'SQL Server 2022', CAST('2033-01-11' AS date))
    ) v (major, product, extended_end)
),
s AS (
    SELECT CAST(SERVERPROPERTY('ProductMajorVersion') AS int) AS major,
           CAST(SERVERPROPERTY('ProductVersion') AS nvarchar(32)) AS build,
           CAST(SERVERPROPERTY('ProductUpdateLevel') AS nvarchar(32)) AS update_level,
           CAST(SERVERPROPERTY('Edition') AS nvarchar(128)) AS edition
)
SELECT l.product,
       s.build,
       s.update_level,
       s.edition,
       l.extended_end AS extended_support_end,
       DATEDIFF(day, GETDATE(), l.extended_end) AS days_left,
       CASE WHEN l.extended_end < GETDATE() THEN 'confirmed' ELSE 'hint' END AS _confidence
FROM s
JOIN lifecycle l ON l.major = s.major
WHERE l.extended_end < DATEADD(day, {{warn_days}}, GETDATE());
