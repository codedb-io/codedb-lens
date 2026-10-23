# MSSQL-BACKUP-001 · Backup ausente ou antigo

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Crítica | Confirmado | `db_datareader` no `msdb` | SQL Server 2017+, Azure SQL MI, RDS (não Azure SQL Database) |

## O que verifica

Para cada banco online (exceto `tempdb` e snapshots), o último backup registrado em `msdb.dbo.backupset`:

- **Nenhum backup full** registrado.
- **Full e diferencial com mais de 8 dias** (`full_max_hours: 192`).
- **Backup de log com mais de 24 horas** (`log_max_hours: 24`) em bancos com recovery `FULL` ou `BULK_LOGGED`.

Backups copy-only são ignorados.

## Por que importa

Sem um backup full recente, uma falha perde tudo desde o último. Em recovery `FULL` sem backup de log,
não há restauração para um ponto no tempo e o log continua crescendo.

## Próxima ação

```sql
-- Histórico recente de um banco
SELECT TOP (20) database_name, type, backup_finish_date
FROM msdb.dbo.backupset WHERE database_name = N'<banco>' ORDER BY backup_finish_date DESC;
```

Verifique os jobs de backup (SQL Agent, Ola Hallengren, sua ferramenta de backup) e faça um backup manual agora.

## Como validar

Quando o backup terminar, confirme no msdb: `SELECT database_name, type, MAX(backup_finish_date) FROM msdb.dbo.backupset GROUP BY database_name, type`. Cada banco deve mostrar um D (full) recente e, em recovery FULL, um L (log) com menos de 24 horas. Rode também um `RESTORE VERIFYONLY` no arquivo.

## Limitações

- Backups de ferramentas que não gravam histórico no `msdb` não aparecem. Nesse caso, o achado é um falso positivo.
- O histórico do `msdb` pode ter sido expurgado (`sp_delete_backuphistory`).
