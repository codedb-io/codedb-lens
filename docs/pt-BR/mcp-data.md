# Uso com dados (MCP)

Com um perfil cujo login lê as suas tabelas, a IA consegue explorar um banco e responder perguntas com SQL: ela lista
as tabelas, descreve as relevantes e roda um `SELECT` que agrega no banco (`GROUP BY`, `TOP`/`LIMIT`) em vez de trazer
linhas cruas. O Lens garante que nada muda; **o login decide o que pode ser lido.** A configuração está em
[Use com IA](mcp.md).

## Um login somente leitura para dados

Dê a ele só o que a IA deve ver. Mantenha-o separado do login de diagnóstico do Lens.

**SQL Server**, um banco:

```sql
CREATE LOGIN lens_ai_data WITH PASSWORD = '<senha>';
USE SalesDB;
CREATE USER lens_ai_data FOR LOGIN lens_ai_data;
ALTER ROLE db_datareader ADD MEMBER lens_ai_data;     -- todas as tabelas, ou então:
-- GRANT SELECT ON SCHEMA::sales TO lens_ai_data;     -- um schema
DENY SELECT ON dbo.customers (email, phone, tax_id) TO lens_ai_data;  -- colunas sensíveis
```

**PostgreSQL**, um banco:

```sql
CREATE ROLE lens_ai_data LOGIN PASSWORD '<senha>';
GRANT CONNECT ON DATABASE sales TO lens_ai_data;
GRANT USAGE ON SCHEMA public TO lens_ai_data;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO lens_ai_data;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO lens_ai_data;  -- tabelas futuras
-- ou, no PostgreSQL 14+, todas as tabelas de todos os schemas: GRANT pg_read_all_data TO lens_ai_data;
```

O PostgreSQL não tem `DENY` por coluna: conceda `SELECT` nas colunas que a IA pode ler, ou entregue uma view sem as
colunas sensíveis.

Salve um perfil com esse login no app (por exemplo `sales-data`) e acrescente ao cliente:

```bash
codedb-lens mcp install --client claude-desktop --profile sales-data
```

## Aponte para uma réplica quando houver

Perguntas analíticas podem varrer tabelas grandes. Num servidor movimentado, use uma réplica de leitura (secundário
legível do Always On, réplica de leitura do RDS/Aurora/Azure, standby do PostgreSQL). Cada consulta também para no
tempo máximo do perfil (10 s por padrão).

## Limites para trabalhar com dados

No app, abra a conexão e depois **Conexão avançada**:

- **Linhas máximas para a IA**: 500 por padrão, até 10.000. Mais linhas consomem mais do contexto da IA; agregar no
  SQL quase sempre é melhor.
- **Tempo máximo da consulta**: 10 s por padrão, até 120 s.

Quando um resultado é cortado (`truncated: true`), a IA é avisada de que a resposta é parcial e de que deve refinar a
consulta.

## Como fica uma conversa

> **Você:** Quais clientes mais compraram neste trimestre no SalesDB?
>
> A IA chama `list_tables`, depois `describe_table` em `orders` e `customers`, depois `query` com `JOIN`, `GROUP BY`
> e `TOP 10`, e responde com uma tabela.
>
> **Você:** Compare com o mesmo trimestre do ano passado, por região.
>
> **Você:** Essa queda no Sul é real ou faltam pedidos? Veja os dias sem pedidos.

Se o login não lê uma tabela, a IA recebe "o login deste perfil não lê este objeto" e avisa você.
