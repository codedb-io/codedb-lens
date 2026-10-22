# Compare and free key

`compare` shows what changed between two runs of the same instance: new findings, resolved findings, findings that are
still open, and rules that started or stopped running. It reads the JSON reports that `check` already saves. It does not
connect to the database or to the internet.

## Get the free key

`compare` needs a free key, valid for 90 days. Renewing it is free and needs no form: from 14 days before the key
expires until 30 days after, run `codedb-lens license renew` or click **Renew key** in the app. After that window,
register again.

- **App:** click **Get the free key** at the bottom of the sidebar.
- **Command line:**

```bash
codedb-lens register                     # asks for each field
codedb-lens register --name "Maria Silva" --email maria@example.com --accept-privacy
```

The key is also sent to your e-mail. On a computer without internet access, activate it from the e-mail:
`codedb-lens license import <key>`, or paste it under **Already have a key?** in the app.

### What is sent, and when

Registering and renewing are the **only** times Lens talks to codeDB, and they happen only when you ask. Lens sends one
request to `https://api.codedb.com.br/lens/register` with:

| Field | Required |
|---|---|
| Name, e-mail | yes |
| Company, role, number of instances you manage | no |
| Lens version and language | yes (used to send the e-mail in your language) |
| Privacy Policy agreement | yes |
| Whether you want news and sales contact | no, unchecked by default |
| Version of the consent text you saw | yes |

Renewing sends only the current key, the Lens version and the consent version to
`https://api.codedb.com.br/lens/renew`; if the Privacy Policy changed since you registered, Lens asks you to accept
it again first.

Nothing about your databases, servers, reports or usage is sent. Sales contact only happens if you check that
option **and** confirm it by e-mail.

On the codeDB side, the IP address is stored truncated (/24 for IPv4, /48 for IPv6). The record is deleted 30 days after
the key expires, or after 24 months without a new registration if you confirmed sales contact. The **Delete my data**
link in the e-mail deletes the whole record immediately. See the
[Privacy Policy](https://app.codedb.com.br/privacidade).

### How the key works

The key is signed with Ed25519 and checked **offline** against a public key built into Lens. Lens never calls home to
validate it. It is stored in the OS credential store (a file readable only by you where there is none, such as Linux
servers). The key does not contain your name or e-mail.

```bash
codedb-lens license            # validity, features and bound instance
codedb-lens license remove     # delete the key from this computer
```

## Compare runs

```bash
codedb-lens compare --profile prod-sql01           # the two latest runs of a profile
codedb-lens compare 2026-10-01_090000.json 2026-10-20_090000.json
```

In the app, open a connection and click **Compare with previous** under *History*.

The result is a self-contained HTML (English/Portuguese) and a JSON, saved next to the newer report as
`compare_<older>_vs_<newer>`.

| Section | Meaning |
|---|---|
| New findings | Was OK, *No data* or absent; now a finding |
| Resolved | Was a finding; now OK |
| Still open | A finding in both runs, with the evidence row count before and after |
| Coverage changes | The rule started or stopped running (permission, version, catalog). **Never counted as resolved** |

Grade and coverage are always shown side by side. Lens warns you when coverage changed by more than 5 points, when the
runs used different Lens versions, or when the rule catalog differs, because the grade change may not reflect the server.

## One instance per key

The free key compares one instance (engine + server name). The first `compare` binds it. To move it to another instance,
run `codedb-lens license reset` (once every 30 days). To follow every instance continuously, see
[codeDB](https://codedb.com.br/?utm_source=lens&utm_medium=docs&utm_campaign=compare).
