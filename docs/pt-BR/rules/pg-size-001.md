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

Para cada tabela: dá para arquivar ou particionar por data as linhas antigas? Todos os índices são usados (`PG-INDEXES-001`)?
Ela está inchada (`PG-BLOAT-001`)?

## Como validar

Meça o tamanho de novo depois de arquivar ou particionar: `SELECT pg_size_pretty(pg_total_relation_size('schema.tabela'))`. O espaço só volta ao sistema operacional após uma reescrita (`DROP` de partições antigas devolve na hora; `DELETE` mais VACUUM não), então verifique as partições removidas ou o resultado do `pg_repack`, não só a contagem de linhas.

## Limitações

Mostra o tamanho atual, não o crescimento: projetar quando o disco enche exige histórico. O espaço livre em disco não é
visível por SQL.
