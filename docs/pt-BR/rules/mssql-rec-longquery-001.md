# MSSQL-REC-LONGQUERY-001 · Queries executando por 1 minuto ou mais

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Confirmado | `VIEW SERVER STATE` | Gravações (`codedb-lens record`): SQL Server 2017+, Azure SQL MI, RDS |

## O que verifica

O coletor `requests` (`sys.dm_exec_requests`, a cada 15 s, sem `WAITFOR`). Um episódio por sessão e query enquanto ela está executando há pelo menos 60 s (`min_s`).

## Por que importa

Requisições executaram por pelo menos um minuto. Queries longas seguram locks e memória e disputam recursos com todo o resto; num sistema online costumam ser a lentidão que o usuário sente.

## Próxima ação

Separe o esperado (backups, manutenção de índices, relatórios à noite) do que não é. Para o resto, veja o plano da query (o texto está no relatório) e os índices faltando e tipos de espera no mesmo período.

## Como validar

Grave de novo no mesmo horário depois da mudança. A query não deve aparecer, ou sua duração de pico deve ser claramente menor.

## Limitações

As amostras são feitas a cada 15 segundos, então a duração real pode ser até 15 segundos maior que o pico mostrado. Rotinas de manutenção também aparecem aqui.
