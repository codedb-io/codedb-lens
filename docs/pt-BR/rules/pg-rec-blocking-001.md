# PG-REC-BLOCKING-001 · Esperas por lock de 30 s ou mais durante a gravação

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado | `pg_monitor` (`pg_read_all_stats`) | Gravações (`codedb-lens record`): PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

O coletor `locks` (`pg_stat_activity` + `pg_blocking_pids()`, a cada 15 s). Um episódio por backend bloqueador enquanto uma sessão espera um lock por pelo menos 30 s (`min_wait_s`).

## Por que importa

Sessões esperaram pelo menos 30 segundos um lock de outro backend. Enquanto dura, os usuários veem timeouts e telas travadas.

## Próxima ação

Comece pelo bloqueador do episódio mais longo. Um bloqueador "idle in transaction" é uma aplicação que abriu uma transação e não terminou; configure idle_in_transaction_session_timeout e corrija a aplicação. Um bloqueador ativo é um comando longo para otimizar ou tirar do horário de pico.

## Como validar

Grave de novo no mesmo horário depois da correção. Os episódios desse bloqueador devem sumir ou ficar bem mais curtos.

## Limitações

As amostras são feitas a cada 15 segundos, então esperas mais curtas podem passar despercebidas, e a duração de um episódio tem precisão de cerca de 15 segundos.
