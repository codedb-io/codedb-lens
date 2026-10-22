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

## Azure SQL Database

Não há login de servidor para conceder: crie um usuário no banco que quer diagnosticar e conecte nesse banco
(preencha **Banco inicial**; no `master` o Lens para com um aviso, porque ali toda DMV fala só do `master`).

```sql
-- rode no banco do usuário
CREATE USER codedb_lens WITH PASSWORD = '<senha forte>';
GRANT VIEW DATABASE STATE TO codedb_lens;
GRANT VIEW DEFINITION TO codedb_lens;
```

Só 6 das 12 regras de SQL Server se aplicam: bloqueios, esperas, top queries, índices ausentes e sem uso, e espaço em
relação ao tamanho máximo do banco. Backups, CHECKDB, SQL Agent, versão, configurações da instância e TempDB são
gerenciados pelo Azure e aparecem como *Não se aplica*. Cada banco do servidor lógico é diagnosticado com a sua conexão.

## PostgreSQL

```sql
CREATE ROLE codedb_lens LOGIN PASSWORD '<senha forte>';
GRANT pg_monitor TO codedb_lens;
GRANT CONNECT ON DATABASE app TO codedb_lens;  -- repita para cada banco
```

O Lens conecta em cada banco, um de cada vez, porque várias estatísticas são por banco.

Opcional: o `PG-BLOAT-001` estima o bloat pelas estatísticas das colunas, que o PostgreSQL só mostra para tabelas em que o
login tem `SELECT`. Sem isso, a regra fica *Sem dados*. Conceda `pg_read_all_data` (14+) ou `SELECT` nas tabelas só se
quiser essa estimativa; todas as outras regras funcionam sem ler dados das tabelas.
