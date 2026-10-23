# MSSQL-UNUSEDIDX-001 · Índices não usados ou duplicados

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Baixa | Indício | `VIEW SERVER STATE` + `VIEW ANY DEFINITION` + `CONNECT ANY DATABASE` | SQL Server 2017+, Azure SQL MI, RDS, Azure SQL Database |

## O que verifica

Roda **em cada banco de usuário** que o login consegue abrir. Para índices nonclustered que não sustentam chave primária
nem constraint unique:

- **Não usado:** nenhum seek, scan ou lookup desde o restart, mas pelo menos 1.000 escritas (`min_writes`), ou seja, só custa.
- **Duplicado:** mesmas colunas de chave, na mesma ordem, e o mesmo filtro de outro índice da tabela.

Bancos que o login não consegue abrir aparecem como ignorados; se nenhum puder ser lido, a regra fica *Sem dados*.

No Azure SQL Database só o banco conectado é verificado (lá não existe `USE`); os contadores de uso também zeram quando o Azure move o banco.

## Por que importa

Cada índice é mantido em todo insert, update e delete, entra no backup e na manutenção. Um índice que ninguém lê é puro custo.

## Próxima ação

Observe durante um ciclo completo do negócio (fechamento de mês, relatórios trimestrais) antes de agir. Gere o script do
índice e desabilite (`ALTER INDEX … DISABLE`) por um tempo antes de remover.

## Como validar

Antes de remover, veja o índice em `sys.dm_db_index_usage_stats` (`user_seeks + user_scans + user_lookups` contra `user_updates`): zero leituras em um ciclo completo de negócio confirma. Depois de remover ou desabilitar, acompanhe a aplicação por alguns dias (consultas lentas, novas sugestões de índice faltando); se algo piorar, recrie o índice pelo script que você guardou. As escritas na tabela devem ficar mais baratas.

## Limitações

- O uso zera no restart. Veja o cartão *Servidor no ar desde* no relatório: poucos dias no ar não bastam.
- Secundárias legíveis e réplicas têm uso próprio; um índice sem uso aqui pode ser usado lá.
