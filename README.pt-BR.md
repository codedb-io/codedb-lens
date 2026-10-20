<p align="center">
  <img src="assets/icon/icon-512.png" width="128" alt="codeDB Lens">
</p>

<h1 align="center">codeDB Lens</h1>

<p align="center">
  <b>Enxergue o que o seu banco não está mostrando.</b><br>
  Diagnóstico gratuito e somente leitura para <b>SQL Server</b> e <b>PostgreSQL</b>.
</p>

<p align="center">
  <a href="README.md">English</a> · Português (Brasil)
</p>

---

> **Status:** em desenvolvimento (v0.1, modo `check`). Ainda sem release.

O codeDB Lens conecta na sua instância com um login **somente leitura**, executa um catálogo de regras
baseadas em evidências e gera um relatório HTML autônomo + JSON com **nota de A a F** e **selo de
cobertura** (quantas regras puderam de fato rodar). Nenhum dado do seu banco sai da sua máquina.

## Por que mais uma ferramenta?

- **Evidência, não opinião.** Cada achado mostra os valores coletados, a query de origem e o horário.
- **Confiança honesta.** Cada achado é *Confirmado*, *Indício* ou *Sem dados*. Regra sem permissão
  nunca aparece como falso "OK"; ela reduz a cobertura.
- **Somente leitura, sempre.** Sem DDL, sem DML, sem agente, sem instalar nada no servidor.
- **Regras são dados.** Cada regra é um YAML + uma query SQL. Contribuir não exige mexer no motor.
- **Inglês e português (Brasil)** no app, na linha de comando e no relatório.

## O que verifica

| SQL Server | PostgreSQL |
|---|---|
| Backup ausente ou antigo | Versão fora de suporte |
| Sem DBCC CHECKDB recente e sem erros | pg_stat_statements indisponível |
| Jobs do SQL Agent com falha | Uma consulta dominando o tempo de execução |
| Disco ou log de transações ficando sem espaço | Transações longas e "idle in transaction" |
| Esperas dominantes, traduzidas | Sessões bloqueadas por locks |
| Sessões bloqueadas neste momento | Risco de wraparound de XID |
| Uma consulta dominando a CPU | Linhas mortas e autovacuum ficando para trás |
| Configurações da instância no padrão | Slots de replicação retendo WAL, réplica atrasada |
| Arquivos do TempDB | Conexões perto do limite, sem pooler |
| Versão fora de suporte | Índices não usados, duplicados e inválidos |
| Índices ausentes de alto impacto | Bloat estimado de tabelas |
| Índices não usados e duplicados | Maiores tabelas |

Cada regra tem uma [página de documentação](docs/pt-BR/rules/) com o que verifica, por que importa, o que fazer e seus limites.

## Início rápido

Windows: execute o instalador (ou o `.zip` portátil) e abra o **codeDB Lens** pelo menu Iniciar.
Escolha o banco, informe o servidor e rode o diagnóstico. Os perfis de conexão ficam salvos; as senhas
vão para o Gerenciador de Credenciais do Windows.

Linha de comando (Windows e Linux):

```bash
codedb-lens check --engine mssql --server prod01 --auth windows
codedb-lens check --engine pg --dsn-env PG_DSN
```

Senhas nunca são aceitas como argumento. Use autenticação integrada, variável de ambiente ou um perfil salvo.

## Permissões mínimas

```sql
-- SQL Server
CREATE LOGIN codedb_lens WITH PASSWORD = '<senha forte>';
GRANT VIEW SERVER STATE TO codedb_lens;
GRANT VIEW ANY DEFINITION TO codedb_lens;
GRANT CONNECT ANY DATABASE TO codedb_lens;   -- regras por banco (índices); não dá acesso a dados
USE msdb;
CREATE USER codedb_lens FOR LOGIN codedb_lens;
ALTER ROLE db_datareader ADD MEMBER codedb_lens;
ALTER ROLE SQLAgentReaderRole ADD MEMBER codedb_lens;
```

```sql
-- PostgreSQL (repita o GRANT CONNECT para cada banco)
CREATE ROLE codedb_lens LOGIN PASSWORD '<senha forte>';
GRANT pg_monitor TO codedb_lens;
GRANT CONNECT ON DATABASE app TO codedb_lens;
```

## Versões suportadas

| Banco | Suportado | Best-effort |
|---|---|---|
| SQL Server | 2017, 2019, 2022, 2025 (Windows e Linux), Azure SQL Managed Instance, Amazon RDS | 2016 e anteriores |
| PostgreSQL | 15, 16, 17, 18, Amazon RDS, Aurora, Azure Flexible Server | 14 |

## Privacidade

Sem telemetria. A única chamada de rede é uma verificação de versão opcional, no máximo uma vez por dia,
no GitHub Releases, sem enviar nenhum dado (`--no-update-check` ou `CODEDB_LENS_NO_UPDATE=1` desligam).
Veja o [SECURITY.md](SECURITY.md).

## Precisa de monitoramento contínuo?

O Lens mostra um retrato do momento. O [**codeDB**](https://codedb.com.br/?utm_source=lens&utm_medium=github&utm_campaign=readme)
monitora suas instâncias SQL Server e PostgreSQL 24 horas por dia, com histórico, baseline e alertas no Teams, Slack e e-mail.

## Como contribuir

Veja o [CONTRIBUTING.md](CONTRIBUTING.md). Novas regras são especialmente bem-vindas.

## Licença

[Apache 2.0](LICENSE). O codeDB SaaS e sua API são produtos separados e de código fechado.
