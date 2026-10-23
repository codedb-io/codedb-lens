# PG-REC-WAITS-001 · Picos de espera durante a gravação

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Indício | `pg_monitor` (`pg_read_all_stats`) | Gravações (`codedb-lens record`): PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

O coletor `activity`: sessões ativas agrupadas por `wait_event_type` e `wait_event` (esperas pelo cliente ficam de fora). Um episódio são duas ou mais amostras seguidas com 5 ou mais sessões no mesmo evento (`min_sessions`).

## Por que importa

Por pelo menos 30 segundos, várias sessões ativas esperavam o mesmo evento (locks, disco, WAL, locks internos). Cada evento aponta um gargalo diferente.

## Próxima ação

Relacione o pico ao tipo de evento: Lock = bloqueio entre sessões, IO (DataFileRead) = leitura de disco (índices ou memória faltando), LWLock = disputa dentro do servidor (WAL, buffers), IO (WALWrite/WALSync) = disco do WAL. Depois veja as top queries do mesmo período.

## Como validar

Grave de novo no mesmo horário depois da mudança. Os picos desse evento devem ficar menores ou mais curtos.

## Limitações

O PostgreSQL mostra só o que cada sessão espera no momento da amostra, não o tempo acumulado, então esperas curtas entre amostras passam despercebidas. Um pico de espera é sintoma, não causa, por isso é um indício.
