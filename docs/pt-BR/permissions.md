# Permissões mínimas

O Lens precisa apenas de leitura de estado e metadados do servidor. Regras sem permissão aparecem como
*Sem dados* e reduzem a cobertura; nunca falham em silêncio.

## SQL Server

```sql
CREATE LOGIN codedb_lens WITH PASSWORD = '<senha forte>';
GRANT VIEW SERVER STATE TO codedb_lens;
GRANT VIEW ANY DEFINITION TO codedb_lens;
GRANT CONNECT ANY DATABASE TO codedb_lens;   -- regras por banco (índices); não dá acesso a dados
USE msdb;
CREATE USER codedb_lens FOR LOGIN codedb_lens;
ALTER ROLE db_datareader ADD MEMBER codedb_lens;
ALTER ROLE SQLAgentReaderRole ADD MEMBER codedb_lens;
```

No SQL Server 2022 em diante, `VIEW SERVER PERFORMANCE STATE` e `VIEW SERVER SECURITY STATE` são as
alternativas granulares ao `VIEW SERVER STATE`.

## PostgreSQL

```sql
CREATE ROLE codedb_lens LOGIN PASSWORD '<senha forte>';
GRANT pg_monitor TO codedb_lens;
GRANT CONNECT ON DATABASE app TO codedb_lens;  -- repita para cada banco
```

O Lens conecta em cada banco, um de cada vez, porque várias estatísticas são por banco.
