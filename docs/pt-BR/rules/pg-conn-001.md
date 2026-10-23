# PG-CONN-001 · Conexões perto do limite, ou ociosas demais

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Indício · Confirmado a 95% do `max_connections` | `pg_monitor` (`pg_read_all_stats`) | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

Conexões de clientes em `pg_stat_activity`:

- **Perto do limite:** 80% ou mais do `max_connections` em uso (`hint_pct`; `confirmed_pct: 95`).
- **Sem pooler:** pelo menos 50 conexões ociosas (`idle_min`) que somam 70% ou mais do total (`idle_pct`).

## Por que importa

No `max_connections`, novas conexões falham e a aplicação para. Cada conexão é um processo com memória própria; centenas
de conexões ociosas desperdiçam RAM e deixam os snapshots mais lentos para todos.

## Próxima ação

Use um pooler (PgBouncer em modo transaction, RDS Proxy, ou o pool da aplicação bem dimensionado) em vez de aumentar o
`max_connections`. Configure também `idle_session_timeout` para clientes que deixam conexões abertas.

## Como validar

Compare as contagens nos horários de pico: `SELECT state, count(*) FROM pg_stat_activity WHERE backend_type = 'client backend' GROUP BY state`, com `SHOW max_connections`. O total deve ficar bem abaixo do limite (menos de 80%) e as ociosas devem cair para um pool pequeno. Checar uma vez em horário calmo não prova nada; confira no período de pico.

## Limitações

Um retrato vê o momento atual; os picos de conexão costumam acontecer em horários específicos.
