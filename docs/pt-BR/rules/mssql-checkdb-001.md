# MSSQL-CHECKDB-001 · Sem DBCC CHECKDB recente e sem erros

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado | `VIEW ANY DEFINITION` | SQL Server 2017+, Azure SQL MI, RDS (não Azure SQL Database) |

## O que verifica

`DATABASEPROPERTYEX(name, 'LastGoodCheckDbTime')` de cada banco online, exceto `tempdb` e snapshots.
O achado aparece quando a última execução sem erros tem mais de 7 dias (`max_days: 7`) ou nunca aconteceu.
Bancos criados dentro da janela são ignorados.

## Por que importa

Corrupção não avisa. Se ela passar despercebida por mais tempo que a retenção dos backups, todos os backups a carregam
e não sobra nada limpo para restaurar.

## Próxima ação

```sql
DBCC CHECKDB (N'<banco>') WITH NO_INFOMSGS, ALL_ERRORMSGS;
```

Agende semanalmente (o `DatabaseIntegrityCheck` do Ola Hallengren é uma escolha comum). Em bancos grandes com janela
curta, rode numa cópia restaurada do backup.

## Como validar

Depois que o DBCC CHECKDB terminar sem erros, confirme a data: `SELECT DATABASEPROPERTYEX(N'banco', 'LastGoodCheckDbTime')` deve ser hoje. Depois confira se o agendamento roda de fato: o valor deve se renovar pelo menos uma vez por semana.

## Limitações

- O `LastGoodCheckDbTime` só registra execuções concluídas sem erros: uma execução que encontrou corrupção não o atualiza.
- Checks rodados em outro servidor, sobre cópias restauradas, não aparecem aqui.
