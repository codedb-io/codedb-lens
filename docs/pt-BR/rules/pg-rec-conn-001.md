# PG-REC-CONN-001 · Conexões perto do max_connections

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado | `pg_monitor` (`pg_read_all_stats`) | Gravações (`codedb-lens record`): PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

O coletor `connections`: conexões de clientes como porcentagem de `max_connections` menos `superuser_reserved_connections`. Um episódio são duas ou mais amostras seguidas com 90% ou mais (`max_pct`).

## Por que importa

As conexões de clientes usaram 90% ou mais das vagas disponíveis por pelo menos 30 segundos. Quando acabam, novas conexões falham com "too many clients" e a aplicação para.

## Próxima ação

Veja quantas dessas conexões estavam ociosas ou idle in transaction. Muitas conexões ociosas pedem um pooler (PgBouncer) ou pools menores na aplicação, não um max_connections maior.

## Como validar

Grave de novo no mesmo horário depois da mudança. As conexões devem ficar bem abaixo do max_connections.

## Limitações

O limite mostrado é max_connections menos superuser_reserved_connections; no PostgreSQL 16+ o reserved_connections também ocupa vagas e não é descontado.
