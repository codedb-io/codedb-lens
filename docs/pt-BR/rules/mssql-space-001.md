# MSSQL-SPACE-001 · Disco ou log de transações ficando sem espaço

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Indício · Confirmado abaixo de 5% livre / acima de 90% de log usado | `VIEW SERVER STATE` (2022+: `VIEW SERVER PERFORMANCE STATE`) | SQL Server 2017+, Azure SQL MI, RDS, Azure SQL Database |

## O que verifica

- **Volumes** com arquivos de banco (`sys.dm_os_volume_stats`) com menos de 15% livre (`volume_hint_pct`).
- **Logs de transações** com mais de 75% usado (`log_hint_pct`, log ativo ÷ tamanho do log via `sys.dm_db_log_stats`) cujo `log_reuse_wait_desc`
  indica algo impedindo a reutilização (qualquer valor diferente de `NOTHING` ou `CHECKPOINT`).

No Azure SQL Database não há volumes: verifica o espaço usado contra o tamanho máximo do banco (`DATABASEPROPERTYEX(..., 'MaxSizeInBytes')`, mesmos limites de 15% / 5%) e o log do banco conectado. Hyperscale não tem tamanho máximo e fica de fora.

## Por que importa

Um volume ou log cheio interrompe as escritas em todos os bancos afetados. Um log que não pode ser reutilizado cresce até chegar a um deles.

## Próxima ação

Trate o `log_reuse_wait_desc`:

| Valor | O que fazer |
|---|---|
| `LOG_BACKUP` | Faça backups de log (ou passe para `SIMPLE` se não precisar de restauração para um ponto no tempo) |
| `ACTIVE_TRANSACTION` | Encontre a transação aberta: `DBCC OPENTRAN` |
| `REPLICATION` / `AVAILABILITY_REPLICA` | Verifique o log reader ou a réplica secundária |

## Como validar

Nos volumes, o espaço livre deve passar dos limites: `SELECT DISTINCT s.volume_mount_point, s.available_bytes / 1048576 AS free_mb FROM sys.master_files f CROSS APPLY sys.dm_os_volume_stats(f.database_id, f.file_id) s`. Nos logs, `SELECT name, log_reuse_wait_desc FROM sys.databases` deve mostrar NOTHING depois do próximo backup de log ou checkpoint, e `DBCC SQLPERF(LOGSPACE)` deve mostrar percentual de uso menor.

## Limitações

Um retrato mostra o quanto está cheio, não a velocidade de crescimento. Projetar quando um volume enche exige histórico.
