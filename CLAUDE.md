# codeDB Lens

Ferramenta gratuita de diagnóstico para SQL Server e PostgreSQL. É o lead magnet do codeDB
(SaaS de monitoramento, Azure, Stripe): útil de verdade, leva o usuário ao SaaS e captura e-mail.
Um binário `codedb-lens` (engine por `--engine mssql|pg`); "for SQL Server" e "for PostgreSQL" são
páginas de docs/marketing. Windows (GUI + CLI) e Linux (CLI).
Repo: github.com/codedb-io/codedb-lens (privado até a v0.5, escrito como público, em inglês).
Plano detalhado: `C:\Users\DESKTOP\.claude\plans\c-users-desktop-downloads-proposta-fina-logical-cloud.md`.

Proposta completa, matriz de validação e análise: https://claude.ai/code/artifact/8dc89e78-12ee-4ac4-b670-5f07795a99c4

## Invariantes (não quebrar)

- **Somente leitura** no banco do usuário. Nenhum DDL/DML, nenhum agente obrigatório.
- **Sem telemetria.** Nenhum dado do banco ou do usuário sai da máquina. Única chamada de rede: checagem
  de versão ≤ 1×/dia (GET anônimo ao GitHub Releases), desligável (`--no-update-check`, `CODEDB_LENS_NO_UPDATE=1`).
- **Regras são dados** (SQL + YAML), não código. Motor + um adaptador por banco. Trocar o motor
  não pode exigir reescrever regras.
- Todo achado tem: ID, título, severidade, confiança (Confirmado / Indício / Sem dados),
  evidência, impacto, próxima ação, link de documentação.
- Nota A–F **sempre** com selo de cobertura (% de regras executadas). Regra sem permissão ou
  sem dados vira "Sem dados", nunca falso "OK".
- Limitar continuidade/escala (24h, 1 instância por chave), **nunca** a qualidade do diagnóstico.
- **Não reaproveitar código do agente do codeDB** (repo MonitorSQL). Este repo será público.
- Permissões mínimas: SQL Server `VIEW SERVER STATE` + `VIEW ANY DEFINITION` + `CONNECT ANY DATABASE` + leitura no msdb;
  PostgreSQL `pg_monitor` + `CONNECT` em cada banco. Regra que precise de mais = rever a regra.

## Stack (MVP)

- **Go** (última estável). Drivers Go puro: `microsoft/go-mssqldb`, `jackc/pgx/v5`. CLI com `flag` da stdlib.
- GUI só no Windows: Wails v2 (WebView2), HTML/JS puro sem npm; o mesmo `.exe` vira CLI quando recebe
  argumentos. Linux: só CLI (servidores/Docker).
- Relatório: `html/template`, HTML autônomo (CSS/JS inline, troca en/pt-BR) + JSON. Sem servidor web local.
- Regras embutidas com `go:embed` em `rules/<engine>/<ID>/{rule.yaml,query.sql}`; `--rules-dir` sobrepõe.
- Perfis em `%APPDATA%\codeDB Lens\profiles.json` / `~/.config/codedb-lens/`; senha só no cofre do SO (`go-keyring`).
- i18n: `en` + `pt-BR`; textos das regras inline no YAML; CI falha se faltar tradução.
- Atualização: `codedb-lens update` verifica `checksums.txt` assinado em Ed25519 (chave pública embutida).
  Só funciona com o repo público (fase 2).
- Segurança das queries: timeout 10 s por regra, execução sequencial, `application_name="codeDB Lens"`,
  MSSQL `LOCK_TIMEOUT`/`DEADLOCK_PRIORITY LOW`, PG `statement_timeout`/`lock_timeout`/read-only.
- Links do SaaS só em `internal/saas/links.go`: landing `https://codedb.com.br/`, cadastro
  `https://app.codedb.com.br/auth/register`, sempre com UTM `utm_source=lens`.
- Ícone: `assets/icon/lens-icon.svg` (mestre) → `python assets/icon/generate.py`. `img/` é só referência
  da marca do SaaS (não versionada).
- Licença Apache 2.0.
- Compatibilidade alvo: SQL Server 2017–2025 (Win/Linux), Azure SQL MI, RDS; PostgreSQL 15–18,
  RDS, Aurora, Azure Flexible. SQL 2016 e PG 14 best-effort.
- Docs: MkDocs Material + `mkdocs-static-i18n` (`docs/en`, `docs/pt-BR`), `mkdocs build --strict`.
- Distribuição: fase 1 GitHub Releases (Inno Setup por usuário + zip; tar.gz Linux x64/arm64) via GoReleaser;
  fase 2 winget, Chocolatey, .deb/.rpm, ghcr.io. Assinatura de código Windows antes da fase 2 (Azure Trusted Signing).
- Azure só a partir da fase 2 (chave Ed25519 assinada no backend do SaaS, chave privada no Key Vault).

## Roadmap

| Fase | Entrega | Critério de saída |
|---|---|---|
| 0 | Validação | Regras do MVP confirmadas, decisões fechadas |
| 1 | v0.1 `check`, 12 regras por banco (uso privado) | CI verde na matriz de versões, testado pelos 2 clientes |
| 2 | v0.5 `compare` + chave grátis (lançamento público) | Cadastro self-service do codeDB no ar |
| 3 | v0.8 `record` (24h) + regras v1 | Overhead medido e documentado |
| 4 | v1.0 `share` + `connect` | Primeiros trials vindos do Lens |

Status (2026-10-20): fase 1 com as 12 + 12 regras do MVP, GUI, relatório, update assinado e pipeline de release
prontos e testados localmente (SQL Server 2017–2025, PostgreSQL 14–18). Falta: repositório no GitHub + secret
`LENS_SIGNING_KEY`, primeira execução do CI, checklist de serviços gerenciados (`RELEASING.md`) e teste com os 2 clientes.

Cadastro (decidido 2026-10-20): nome e e-mail obrigatórios + empresa opcional, pedidos só ao ativar a chave
grátis na fase 2 (`compare`/`record`), enviados ao backend do codeDB. O `check` da v0.1 segue sem cadastro.

Em aberto: validade da chave (90/30 dias), elegibilidade do Azure Trusted Signing, overhead máximo do `record`.

---

## graphify

Quando existir `graphify-out/graph.json`:
- Para perguntas sobre o código, rode primeiro `graphify query "<pergunta>"`; `graphify path "<A>" "<B>"`
  para relações e `graphify explain "<conceito>"` para conceitos.
- Se `graphify-out/wiki/index.md` existir, use-o para navegação ampla.
- Leia `graphify-out/GRAPH_REPORT.md` só para revisão de arquitetura.
- Depois de alterar código, rode `graphify update .` (só AST, sem custo de API).

## Distribuição de modelos

- Opus: arquitetura, bugs complexos e revisão de código.
- Sonnet: edições, testes, documentação e refatoração.
- Haiku: pesquisas e resumos.
- Especifique o modelo em cada chamada de agente.

## Delegação

- Use a skill `/grilling` para fechar decisões antes de implementar.
- Um subagente por tarefa. Planeje antes de executar.
- Execute subagentes independentes em paralelo.
- Leia o relatório, nunca os arquivos.
