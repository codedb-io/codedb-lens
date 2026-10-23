# Why Windows shows a warning

When you run the codeDB Lens installer or `codedb-lens.exe` for the first time, Windows may show **"Windows protected
your PC"** (SmartScreen) or say the publisher is unknown. This page explains why, what is safe to do and how to check
that the file is the one we built.

## Why it happens

Lens is open source and its Windows binaries are **not code-signed yet**. A code-signing certificate is a paid product
(recurring, and priced for companies), and free programs for open-source projects require a public project with some
history, which Lens did not have when it applied. We would rather say this plainly than hide it.

Even a signed file gets a SmartScreen warning until it builds "reputation" through many downloads. Unsigned files start
with none. The warning means *Windows doesn't know this file yet*. It does not mean the file was found to be harmful.

## What is safe to do

On the SmartScreen dialog, click **More info**, then **Run anyway**. Do this only after checking the file as described
below, and only for a file you downloaded from the
[GitHub Releases page](https://github.com/codedb-io/codedb-lens/releases/latest) of this project.

## Verify your download

Every release publishes `checksums.txt` with the SHA-256 of each file.

1. Download `checksums.txt` from the same release.
2. Compute the hash of your file in a terminal:

    ```bat
    certutil -hashfile codedb-lens_windows_amd64_setup.exe SHA256
    ```

3. Compare it with the line for that file in `checksums.txt`. They must be identical.

Two stronger checks:

- **Build provenance (from the release after v0.6.0).** Each asset has a GitHub build attestation that ties the file to
  this repository and to the workflow run that built it. With the [GitHub CLI](https://cli.github.com/):

    ```bash
    gh attestation verify codedb-lens_windows_amd64_setup.exe --repo codedb-io/codedb-lens
    ```

- **Signed checksums.** `checksums.txt.sig` is an Ed25519 signature of `checksums.txt`. You don't need to check it by
  hand: `codedb-lens update` verifies it automatically, with the public key built into the program, before replacing
  anything.

## Alternatives without the warning

- **Scoop:** `scoop bucket add codedb https://github.com/codedb-io/codedb-lens`, then `scoop install codedb/codedb-lens`.
  Scoop checks the hash on download.
- **Docker:** `docker run --rm -e PG_DSN -v "$PWD:/out" ghcr.io/codedb-io/codedb-lens check --engine pg --dsn-env PG_DSN --out /out`.
- **Linux packages:** `.deb`, `.rpm` or the `.tar.gz` from the release page, if you can run Lens from a Linux host or WSL.
- **Build from source:** with [Go](https://go.dev/dl/) installed, `git clone https://github.com/codedb-io/codedb-lens`
  and `go build ./cmd/codedb-lens` in the repository folder give you a binary you compiled yourself. The CLI is
  all you need for `check`; the desktop app is built with Wails (see the repository for the build steps).

## Corporate environments

Some managed PCs block unsigned programs outright, with no "Run anyway" button: **AppLocker**, **Windows Defender
Application Control (WDAC)** or **Smart App Control**. In that case:

- run Lens from Docker or from a Linux host/WSL that is allowed to reach the database; or
- ask IT to allow the file **by hash**: give them the SHA-256 from `checksums.txt` for the version you want. Allow-by-hash
  needs no certificate.

## Transparency

- Source code: [github.com/codedb-io/codedb-lens](https://github.com/codedb-io/codedb-lens) (Apache 2.0).
- The release is built by GitHub Actions from a tag, not on a personal machine:
  [`.github/workflows/release.yml`](https://github.com/codedb-io/codedb-lens/blob/main/.github/workflows/release.yml).
- What Lens sends over the network is listed in [Privacy](privacy.md): no telemetry, and the database is only ever read.

If you find something suspicious, see [SECURITY.md](https://github.com/codedb-io/codedb-lens/blob/main/SECURITY.md).
