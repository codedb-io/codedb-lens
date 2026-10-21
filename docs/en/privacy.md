# Security and privacy

- **Read-only.** Only catalog, DMV and statistics queries. Sessions use lock/statement timeouts and
  identify themselves as `codeDB Lens`.
- **No telemetry.** Nothing about your database, servers or usage leaves your machine.
- **One optional network call:** at most once a day, an anonymous request to GitHub Releases to check for
  a new version. Turn it off with `--no-update-check` or `CODEDB_LENS_NO_UPDATE=1`.
- **Free key registration, only when you ask:** `codedb-lens register` (or **Get the free key** in the app) sends the
  name and e-mail you type, once, to codeDB. Nothing about your databases. The key is then checked offline.
  See [Compare and free key](compare.md#what-is-sent-and-when).
- **Signed updates:** `codedb-lens update` verifies an Ed25519 signature before replacing the binary.
- **Passwords** are stored only in the OS credential store, never in plain text and never as arguments.
- **Reports** can contain server names and query text. Treat them as internal documents.
