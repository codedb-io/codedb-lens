# PG-REC-LONGQUERY-001 · Queries executando por 1 minuto ou mais

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Confirmado | `pg_monitor` (`pg_read_all_stats`) | Gravações (`codedb-lens record`): PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

O coletor `activity` (`pg_stat_activity` no estado `active`, a cada 15 s). Um episódio por backend e query enquanto o comando está executando há pelo menos 60 s (`min_s`).

## Por que importa

Comandos executaram por pelo menos um minuto. Comandos longos seguram locks, impedem o autovacuum de limpar versões antigas das linhas e disputam recursos com todo o resto.

## Próxima ação

Separe o esperado (backups, VACUUM, relatórios à noite) do que não é. Para o resto, rode EXPLAIN (ANALYZE, BUFFERS) na query (o texto está no relatório) e considere statement_timeout para o usuário da aplicação.

## Como validar

Grave de novo no mesmo horário depois da mudança. A query não deve aparecer, ou sua duração de pico deve ser claramente menor.

## Limitações

As amostras são feitas a cada 15 segundos, então a duração real pode ser até 15 segundos maior que o pico mostrado. Comandos de manutenção também aparecem aqui.
