# MCP com Docker

Num servidor Linux, no CI ou onde não houver perfis salvos, rode o servidor MCP pela imagem. A conexão vem de flags e
variáveis de ambiente; a senha, de `CODEDB_LENS_PASSWORD` ou, no PostgreSQL, de uma string de conexão numa variável de
ambiente. Ela nunca é digitada: o stdin é o canal do protocolo.

```bash
docker run -i --rm --network host -e CODEDB_LENS_PASSWORD \
  ghcr.io/codedb-io/codedb-lens mcp --engine pg --server db01 --user lens --database app
```

O `-i` é obrigatório: o cliente conversa com o servidor pelo stdin e pelo stdout. O `--network host` deixa o container
alcançar um banco na rede do host; nos outros casos, use uma rede Docker.

## Claude Code

```bash
claude mcp add --scope user -e CODEDB_LENS_PASSWORD=<senha> codedb-lens -- \
  docker run -i --rm --network host -e CODEDB_LENS_PASSWORD ghcr.io/codedb-io/codedb-lens \
  mcp --engine mssql --server sql01 --user lens --trust-server-cert
```

## Claude Desktop, VS Code, Cursor

```json
{
  "mcpServers": {
    "codedb-lens": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "--network", "host", "-e", "PG_DSN",
               "ghcr.io/codedb-io/codedb-lens", "mcp", "--engine", "pg", "--dsn-env", "PG_DSN"],
      "env": { "PG_DSN": "postgres://lens:<senha>@db01:5432/app" }
    }
  }
}
```

O VS Code usa a chave `servers` em vez de `mcpServers`, e cada entrada também leva `"type": "stdio"`.

## Observações

- A conexão por flags é um perfil só, chamado `default`. O `check` salva o relatório dentro do container, que o
  `--rm` descarta; a IA recebe os resultados diretamente.
- As gravações são lidas de arquivos, então o `list_recordings` não encontra nenhuma num container novo; rode o
  `codedb-lens record` com uma pasta montada se quiser que a IA leia uma.
- A imagem nunca se atualiza sozinha: baixe uma tag nova.
