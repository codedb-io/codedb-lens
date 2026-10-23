# MSSQL-WAITS-001 · Instância esperando muito por um recurso

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Indício · Confirmado para `THREADPOOL` / `RESOURCE_SEMAPHORE` | `VIEW SERVER STATE` (2022+: `VIEW SERVER PERFORMANCE STATE`) | SQL Server 2017+, Azure SQL MI, RDS, Azure SQL Database |

## O que verifica

`sys.dm_os_wait_stats`, contando só esperas com significado claro (esperas ociosas e de segundo plano nunca entram),
agrupadas pelo que significam. Uma categoria aparece quando representa pelo menos 20% dessas esperas (`min_share_pct`)
**e** a instância esperou por ela pelo menos 5 minutos por hora no ar (`min_wait_s_per_hour: 300`).

No Azure SQL Database lê `sys.dm_db_wait_stats` (só este banco) e também conta o limite de escrita de log do nível de serviço (`*LOG_RATE_GOVERNOR`): se ele domina, o nível é pequeno para a carga de escrita.

## O que cada espera costuma significar

| Significado | Tipos de espera | Causas comuns |
|---|---|---|
| Disco: leitura de páginas | `PAGEIOLATCH_*` | Pouca memória para o volume de dados em uso, índices faltando causando scans, storage lento |
| Disco: escrita no log | `WRITELOG` | Disco de log lento, muitas transações pequenas (commit por linha) |
| Bloqueio | `LCK_M_*` | Transações longas, índices faltando em updates, nível de isolamento |
| Disputa de páginas em memória | `PAGELATCH_*` | Disputa de alocação no TempDB, inserts concentrados na última página |
| Pressão de CPU | `SOS_SCHEDULER_YIELD` | Consultas caras, poucas CPUs |
| Paralelismo | `CXPACKET`, `CXCONSUMER` | MAXDOP / cost threshold no padrão, planos paralelos desbalanceados |
| Memória para consultas | `RESOURCE_SEMAPHORE` | Consultas pedindo muita memória para sort/hash, estimativas ruins |
| Falta de threads | `THREADPOOL` | Cadeias de bloqueio, paralelismo demais. Grave: novas conexões podem falhar |
| Aplicação lendo devagar | `ASYNC_NETWORK_IO` | Aplicação processando linha a linha, ou a rede |

## Próxima ação

Comece pela primeira categoria e suas causas comuns. Compare com `MSSQL-TOPQUERY-001` e `MSSQL-BLOCKING-001` no mesmo relatório.

## Como validar

As esperas acumulam desde o último restart, então a correção só aparece como mudança ao longo do tempo. Colete duas vezes, com algumas horas de intervalo, e compare: `SELECT wait_type, wait_time_ms FROM sys.dm_os_wait_stats WHERE wait_type = N'a_espera'`. O crescimento por hora dessa espera deve ficar claramente menor que antes da mudança. Em instância de teste dá para zerar os contadores com `DBCC SQLPERF('sys.dm_os_wait_stats', CLEAR)`.

## Limitações

Os totais acumulam desde o restart. Mostram o perfil de longo prazo, não o que acontece agora; só a diferença entre dois
momentos mostra o presente.
