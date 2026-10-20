# MSSQL-WAITS-001 · Instância esperando muito por um recurso

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Indício · Confirmado para `THREADPOOL` / `RESOURCE_SEMAPHORE` | `VIEW SERVER STATE` (2022+: `VIEW SERVER PERFORMANCE STATE`) | SQL Server 2017+, Azure SQL MI, RDS |

## O que verifica

`sys.dm_os_wait_stats`, contando só esperas com significado claro (esperas ociosas e de segundo plano nunca entram),
agrupadas pelo que significam. Uma categoria aparece quando representa pelo menos 20% dessas esperas (`min_share_pct`)
**e** a instância esperou por ela pelo menos 5 minutos por hora no ar (`min_wait_s_per_hour: 300`).

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

## Limitações

Os totais acumulam desde o restart. Mostram o perfil de longo prazo, não o que acontece agora; só a diferença entre dois
momentos mostra o presente.
