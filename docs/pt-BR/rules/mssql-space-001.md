# MSSQL-SPACE-001 · Disco ou log de transações ficando sem espaço

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Indício · Confirmado abaixo de 5% livre / acima de 90% de log usado | `VIEW SERVER STATE` (2022+: `VIEW SERVER PERFORMANCE STATE`) | SQL Server 2017+, Azure SQL MI, RDS |

## O que verifica

- **Volumes** com arquivos de banco (`sys.dm_os_volume_stats`) com menos de 15% livre (`volume_hint_pct`).
- **Logs de transações** com mais de 75% usado (`log_hint_pct`, log ativo ÷ tamanho do log via `sys.dm_db_log_stats`) cujo `log_reuse_wait_desc`
  indica algo impedindo a reutilização (qualquer valor diferente de `NOTHING` ou `CHECKPOINT`).

## Por que importa

Um volume ou log cheio interrompe as escritas em todos os bancos afetados. Um log que não pode ser reutilizado cresce até chegar a um deles.

## Próxima ação

Trate o `log_reuse_wait_desc`:

| Valor | O que fazer |
|---|---|
| `LOG_BACKUP` | Faça backups de log (ou passe para `SIMPLE` se não precisar de restauração para um ponto no tempo) |
| `ACTIVE_TRANSACTION` | Encontre a transação aberta: `DBCC OPENTRAN` |
| `REPLICATION` / `AVAILABILITY_REPLICA` | Verifique o log reader ou a réplica secundária |

## Limitações

Um retrato mostra o quanto está cheio, não a velocidade de crescimento. Projetar quando um volume enche exige histórico.
