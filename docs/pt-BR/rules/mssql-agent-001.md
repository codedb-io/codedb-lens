# MSSQL-AGENT-001 · Jobs do SQL Agent com falha

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Alta | Confirmado | `SQLAgentReaderRole` no `msdb` | SQL Server 2017+ (não Express), Azure SQL MI, RDS |

## O que verifica

Jobs habilitados em `msdb.dbo.sysjobs` cujo **último** resultado (`sysjobhistory`, `step_id = 0`) foi falha ou
cancelamento nos últimos 7 dias (`days: 7`). A evidência mostra também quantas vezes cada job falhou nessa janela.

## Por que importa

A maioria dos jobs existe para manter algo funcionando: backups, manutenção de índices, cargas de dados. Um job com
falha costuma ser o primeiro sinal de que uma dessas tarefas parou sem ninguém perceber.

## Próxima ação

```sql
EXEC msdb.dbo.sp_help_jobhistory @job_name = N'<job>', @mode = N'FULL';
```

Corrija o passo que falha e configure um operador com `@notify_level_email = 2` (em caso de falha).

## Como validar

Execute o job manualmente (`EXEC msdb.dbo.sp_start_job N'nome do job'`) e confirme no histórico que o último resultado é Succeeded. Depois espere a próxima execução agendada: o job deixa de aparecer aqui quando o resultado mais recente é sucesso.

## Limitações

- Um job que se recuperou (última execução com sucesso) não aparece, mesmo que tenha falhado antes na janela.
- O histórico é limitado pela retenção configurada no Agent.
- Não existe no Express nem no Azure SQL Database, que não têm SQL Agent.
