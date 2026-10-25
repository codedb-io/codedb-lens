# Use with data (MCP)

With a profile whose login reads your tables, the AI can explore a database and answer questions with SQL: it lists
the tables, describes the relevant ones, and runs one `SELECT` that aggregates in the database (`GROUP BY`,
`TOP`/`LIMIT`) instead of fetching raw rows. The Lens guarantees nothing changes; **the login decides what can be
read.** Setup is in [Use with AI](mcp.md).

## A read-only login for data

Give it only what the AI should see. Keep it apart from the Lens's diagnosis login.

**SQL Server**, one database:

```sql
CREATE LOGIN lens_ai_data WITH PASSWORD = '<password>';
USE SalesDB;
CREATE USER lens_ai_data FOR LOGIN lens_ai_data;
ALTER ROLE db_datareader ADD MEMBER lens_ai_data;     -- every table, or instead:
-- GRANT SELECT ON SCHEMA::sales TO lens_ai_data;     -- one schema
DENY SELECT ON dbo.customers (email, phone, tax_id) TO lens_ai_data;  -- sensitive columns
```

**PostgreSQL**, one database:

```sql
CREATE ROLE lens_ai_data LOGIN PASSWORD '<password>';
GRANT CONNECT ON DATABASE sales TO lens_ai_data;
GRANT USAGE ON SCHEMA public TO lens_ai_data;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO lens_ai_data;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO lens_ai_data;  -- future tables
-- or, PostgreSQL 14+, every table of every schema: GRANT pg_read_all_data TO lens_ai_data;
```

PostgreSQL has no column-level `DENY`: grant `SELECT` on the columns the AI may read, or give it a view without the
sensitive ones.

Save a profile with this login in the app (for example `sales-data`) and add it to the client:

```bash
codedb-lens mcp install --client claude-desktop --profile sales-data
```

## Point it at a replica when you have one

Analytical questions can scan large tables. On a busy server, use a read replica (an Always On readable secondary,
an RDS/Aurora/Azure read replica, a PostgreSQL standby). Each query also stops at the profile's time limit (10 s by
default).

## Limits for data work

In the app, open the connection, then **Advanced connection**:

- **Max rows for the AI**: 500 by default, up to 10,000. More rows cost more of the AI's context; aggregating in
  SQL is almost always better.
- **Max query time**: 10 s by default, up to 120 s.

When a result is cut (`truncated: true`), the AI is told the answer is partial and to refine the query.

## What a conversation looks like

> **You:** Which customers bought the most this quarter in SalesDB?
>
> The AI calls `list_tables`, then `describe_table` on `orders` and `customers`, then `query` with a `JOIN`,
> `GROUP BY` and `TOP 10`, and answers with a table.
>
> **You:** Compare with the same quarter last year, by region.
>
> **You:** Is that drop in the South real, or are orders missing? Check the days with no orders.

If the login cannot read a table, the AI gets "the login of this profile cannot read this object" and tells you.
