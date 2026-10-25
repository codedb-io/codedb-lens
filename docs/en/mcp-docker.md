# MCP with Docker

On a Linux server, in CI or wherever there are no saved profiles, run the MCP server from the image. The connection
comes from flags and environment variables; the password from `CODEDB_LENS_PASSWORD` or, for PostgreSQL, a connection
string in an environment variable. It is never typed: stdin is the protocol channel.

```bash
docker run -i --rm --network host -e CODEDB_LENS_PASSWORD \
  ghcr.io/codedb-io/codedb-lens mcp --engine pg --server db01 --user lens --database app
```

`-i` is required: the client talks to the server over stdin and stdout. `--network host` lets the container reach a
database on the host's network; use a Docker network otherwise.

## Claude Code

```bash
claude mcp add --scope user -e CODEDB_LENS_PASSWORD=<password> codedb-lens -- \
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
      "env": { "PG_DSN": "postgres://lens:<password>@db01:5432/app" }
    }
  }
}
```

VS Code uses the key `servers` instead of `mcpServers`, and each entry also takes `"type": "stdio"`.

## Notes

- The connection from flags is one profile, named `default`. `check` saves its report inside the container, which
  `--rm` discards; the AI gets the results directly.
- Recordings are read from files, so `list_recordings` finds none in a fresh container; run `codedb-lens record` with
  a mounted folder if you want the AI to read one.
- The image never updates itself: pull a new tag.
