# MSSQL-DBOPTIONS-001 · Bancos com opções de risco

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Confirmado | `VIEW ANY DEFINITION` | SQL Server 2017+, Azure SQL MI, RDS (não Azure SQL Database) |

## O que verifica

`sys.databases` de cada banco de usuário online (`database_id > 4`, sem snapshots). O achado aparece quando
`AUTO_SHRINK` ou `AUTO_CLOSE` está ligado, ou quando `PAGE_VERIFY` é diferente de `CHECKSUM`.

## Por que importa

- **AUTO_SHRINK** encolhe os arquivos em segundo plano, fragmenta índices e depois briga com o autogrowth, que os faz crescer de novo.
- **AUTO_CLOSE** fecha o banco quando a última conexão sai, então a próxima conexão paga o custo de abri-lo.
- **PAGE_VERIFY diferente de CHECKSUM** (`TORN_PAGE_DETECTION` ou `NONE`) deixa páginas danificadas passarem despercebidas até alguém lê-las, muitas vezes depois de o dano chegar aos backups.

## Próxima ação

```sql
ALTER DATABASE [banco] SET AUTO_SHRINK OFF, AUTO_CLOSE OFF;
ALTER DATABASE [banco] SET PAGE_VERIFY CHECKSUM;
```

As páginas só recebem checksum quando são gravadas de novo, então faça um backup completo e rode `DBCC CHECKDB` depois.

## Como validar

```sql
SELECT name, is_auto_shrink_on, is_auto_close_on, page_verify_option_desc
FROM sys.databases WHERE database_id > 4;
```

As duas flags devem ser `0` e `page_verify_option_desc` deve ser `CHECKSUM`.

## Limitações

- Bancos de sistema não são verificados.
- Não se aplica ao Azure SQL Database, onde a plataforma gerencia essas opções.
- Alguns aplicativos de fornecedores pedem `AUTO_CLOSE` no SQL Server Express; confirme com o fornecedor antes de alterar.
