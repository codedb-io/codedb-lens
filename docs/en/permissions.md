# Minimum permissions

Lens needs only read access to server state and metadata. Rules that lack a permission report
*No data* and lower the coverage; they never fail silently.

## SQL Server

```sql
CREATE LOGIN codedb_lens WITH PASSWORD = '<strong password>';
GRANT VIEW SERVER STATE TO codedb_lens;
GRANT VIEW ANY DEFINITION TO codedb_lens;
GRANT CONNECT ANY DATABASE TO codedb_lens;   -- per-database rules (indexes); no data access
USE msdb;
CREATE USER codedb_lens FOR LOGIN codedb_lens;
ALTER ROLE db_datareader ADD MEMBER codedb_lens;
ALTER ROLE SQLAgentReaderRole ADD MEMBER codedb_lens;
```

On SQL Server 2022 and later, `VIEW SERVER PERFORMANCE STATE` and `VIEW SERVER SECURITY STATE` are the
granular alternatives to `VIEW SERVER STATE`.

## PostgreSQL

```sql
CREATE ROLE codedb_lens LOGIN PASSWORD '<strong password>';
GRANT pg_monitor TO codedb_lens;
GRANT CONNECT ON DATABASE app TO codedb_lens;  -- repeat for each database
```

Lens connects to each database in turn, because several statistics are per database.
