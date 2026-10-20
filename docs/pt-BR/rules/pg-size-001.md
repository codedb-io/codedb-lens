# PG-SIZE-001 · Tabelas grandes que dominam o espaço

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Baixa | Indício | `pg_monitor` + `CONNECT` em cada banco | PostgreSQL 14+, RDS, Aurora, Azure Flexible |

## O que verifica

Roda **em cada banco** em que o login consegue conectar. Lista até 10 tabelas e materialized views com pelo menos 10 GB
(`min_rel_gb`), incluindo índices e TOAST, com a participação no banco. Bancos sem `CONNECT` aparecem como ignorados.

## Por que importa

Um punhado de tabelas grandes costuma puxar o crescimento do disco, a duração do backup e o tempo de vacuum. É nelas que
arquivamento, particionamento e limpeza de índices compensam.

## Próxima ação

Para cada tabela: dá para arquivar ou particionar por data as linhas antigas? Todos os índices são usados (`PG-UNUSEDIDX`,
no próximo lote)? Ela está inchada (`PG-BLOAT`)?

## Limitações

Mostra o tamanho atual, não o crescimento: projetar quando o disco enche exige histórico. O espaço livre em disco não é
visível por SQL.
