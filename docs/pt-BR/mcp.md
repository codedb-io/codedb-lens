# Use com IA (MCP)

Pergunte ao Claude, ao GitHub Copilot ou ao Cursor sobre o seu SQL Server ou PostgreSQL em linguagem natural. O
`codedb-lens mcp` é um servidor [Model Context Protocol](https://modelcontextprotocol.io/): o seu cliente de IA o
inicia no seu computador e chama as ferramentas do Lens. **Ele só lê.** Nenhuma porta é aberta e nada é instalado no
servidor.

Dois usos, definidos pelo login do perfil que você entrega à IA:

- **Diagnóstico**: o login mínimo do Lens ([Permissões](permissions.md)). A IA roda o diagnóstico, lê gravações e
  explica o que corrigir. Ela não consegue ler dados de negócio.
- **Dados**: um login que lê as tabelas que você escolher ([Uso com dados](mcp-data.md)). A IA lista as tabelas,
  descreve-as e roda `SELECT`s para responder perguntas, comparar períodos ou montar tabelas e gráficos no chat.

> **Privacidade.** O Lens não faz nenhuma chamada de rede. O seu cliente de IA envia o que o Lens devolve (achados e,
> quando o login lê tabelas, linhas de dados) ao provedor do modelo. Mascare nomes com `--redact` e entregue à IA um
> login sem acesso a colunas sensíveis.

## 1. Antes de começar

- codeDB Lens instalado, por qualquer canal ([Primeiros passos](getting-started.md)).
- Um perfil de conexão salvo, com a senha: crie no app ou com `codedb-lens check ... --save NOME`. A IA nunca vê
  senhas.
- Para um servidor Linux ou CI sem perfis, veja [MCP com Docker](mcp-docker.md).

## 2. Adicione o Lens ao seu cliente

O app mostra estes comandos, prontos para copiar, na tela **IA (MCP)**. O instalador não coloca o Lens no `PATH` por
padrão, por isso os comandos usam o caminho completo, no PowerShell (o padrão do Terminal do Windows):

```powershell
& "$env:LOCALAPPDATA\Programs\codeDB Lens\codedb-lens.com" mcp install --client claude-desktop --profile prod-diag
```

| Cliente | `--client` | O que acontece |
|---|---|---|
| Claude Desktop | `claude-desktop` | Acrescenta o Lens ao `claude_desktop_config.json` |
| VS Code (Copilot) | `vscode` | Acrescenta o Lens ao `mcp.json` do usuário |
| Cursor | `cursor` | Acrescenta o Lens ao `~/.cursor/mcp.json` |
| Claude Code | `claude-code` | Imprime o comando `claude mcp add` para rodar; o Claude Code gerencia a própria configuração |

O `mcp install` altera só a entrada `codedb-lens`, mantém todo o resto, salva o arquivo anterior como `.bak` e não faz
nada numa segunda execução. `--dry-run` mostra a alteração sem gravar. Se o arquivo não for JSON válido, ele fica como
está e o comando imprime o trecho para colar à mão. **Reinicie o cliente** depois.

Opções:

- `--profile a,b`: os perfis que a IA pode usar (padrão: todos os salvos). Prefira nomeá-los.
- `--redact`: mascara nomes de host, banco, objeto, login e arquivo nos resultados de diagnóstico, comparação e
  gravação.

`codedb-lens mcp status` mostra quais clientes têm o Lens e se o caminho gravado ainda existe.
`mcp uninstall --client ...` remove a entrada.

## 3. Pergunte

- "Rode o diagnóstico em prod-diag e diga o que corrigir primeiro."
- "O que causou a lentidão de ontem às 14h? Use a gravação."
- "Quais tabelas existem no SalesDB?"
- "Top 10 clientes por faturamento no trimestre."
- "Compare os pedidos deste mês com o mesmo mês do ano passado."

O cliente também lista três prompts: **diagnose**, **incident** e **explore_data**.

## Ferramentas

| Ferramenta | O que faz |
|---|---|
| `list_profiles` | Perfis que a IA pode usar: nome, engine, host, banco inicial. Nunca senhas nem strings de conexão |
| `check` | Roda o diagnóstico (10–60 s): nota com cobertura, achados com evidência; salva o relatório no histórico do perfil, como o app |
| `get_finding` | Uma regra de uma execução salva, com todas as linhas de evidência |
| `history` / `compare` | Execuções salvas; achados novos, resolvidos e persistentes entre duas execuções (o compare exige a [chave grátis](compare.md)) |
| `list_rules` / `explain_rule` | O catálogo de regras: o que cada uma mede, o SQL e a documentação |
| `list_recordings` / `recording_report` | Gravações e o relatório de um período ([Gravar ao longo do tempo](record.md)) |
| `record_command` | O comando `codedb-lens record` para rodar num terminal: a gravação dura mais que o chat, então a IA nunca inicia uma |
| `list_databases` / `list_tables` / `describe_table` | O que o login enxerga: bancos, tabelas e views, colunas, chaves e índices |
| `query` | Um `SELECT` somente leitura, com limites |

## Limites

O `query` executa um `SELECT` (ou `WITH ... SELECT`) e recusa todo o resto: escritas, `EXEC`, várias instruções,
`SELECT INTO`, funções com efeito colateral. Ele roda numa transação que é sempre desfeita (no PostgreSQL, também
somente leitura), para no teto de linhas do perfil (500 por padrão, células cortadas em 2 KB) e no tempo máximo
(10 s). Um resultado cortado vem com `truncated: true`, e a IA é orientada a refinar a consulta. Ajuste os limites por
conexão no app (**Conexão avançada** → *Linhas máximas para a IA*, *Tempo máximo da consulta*), até 10.000 linhas e 120 s.

No SQL Server, o `query` lê dados não confirmados, como o diagnóstico, para nunca esperar por quem está escrevendo.

## Problemas comuns

| Problema | O que fazer |
|---|---|
| O cliente não mostra as ferramentas do Lens | Reinicie o cliente; rode `codedb-lens mcp status` |
| Parou de funcionar depois de atualizar | O caminho mudou: rode o comando `mcp install` de novo |
| "não há senha salva para o perfil" | Salve a senha no app ou defina `CODEDB_LENS_PASSWORD` na configuração do servidor no cliente |
| "o login do perfil ... não lê este objeto" | O login do perfil não tem `SELECT` nessa tabela: conceda ou use outro perfil |
| "perfil ... não liberado para esta sessão de IA" | Ele não está no `--profile`: acrescente e rode o `mcp install` de novo |
