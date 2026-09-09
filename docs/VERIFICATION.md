# Verification

## 3.0.1 local validation — 2026-09-09

- Formatting, vet, race tests, native build, and temporary OpenSSL material tests passed.
- Real PTY tests (`uv run --with pexpect --with pyte python scripts/test_terminal.py`)
  passed: Ctrl+C/Esc/Ctrl+D, external SIGINT, Korean editing, arrows/Delete/Backspace,
  long input, resize, multiple prompts, and canonical/echo terminal restoration.
- Tests use temporary HOME directories and synthetic settings; no live OAuth or orders.
- Functional help group membership is covered by Go tests. PTY checks also run in CI/release.

## 2.0.0 local validation — 2026-09-09

- Formatting, `go vet ./...`, `go test -race ./...`, and native build passed.
- Tests cover date inclusivity, DST boundaries, range merging and failure handling,
  interactive inputs, renamed VWAP command, and explicit PostgreSQL opt-in.
- Live IBKR requests and production PostgreSQL integration remain untested.

## 2.0.0 release and server installation — 2026-09-09

- GitHub release workflow 34316469688 passed formatting, vet, race tests and both builds.
- Published `v2.0.0` with Linux amd64/macOS arm64 archives and SHA-256 checksums.
- Updated `~/tools/deploy-ibkr.sh` on the `airflow` SSH host and ran it with `2.0.0`.
- Installer verified the archive checksum and installed `~/.local/bin/ibkr`.
- Remote `--version` returned `ibkr 2.0.0`; history and VWAP help commands passed,
  displaying the renamed command, date options, and global `--database` flag.
- These remote checks did not call IBKR or connect to PostgreSQL.

## Initial migration validation — 2026-09-08 (before publishing)

- Go runtime used: go1.27.0, macOS arm64.
- `go vet ./...`: passed.
- `gofmt` check: passed.
- `go test -race ./... -count=1` with the Rust reference enabled: 57 tests/subtests passed.
- 22 compatible command cases compared against the existing Rust 0.10.1 executable:
  HTTP methods, paths, query parameters, JSON bodies, JSON outputs matched.
  The existing executable was used; Rust was not rebuilt during this migration.
- Real OpenSSL integration: generated all seven files in a temporary directory, validated
  all four RSA private-key representations and DH parameters, then removed temporary files.
- Six standalone binary checks passed: version, short version, help, unknown command failure,
  missing-credentials failure, and offline fee-plan JSON.
- Native macOS arm64 binary built at `bin/ibkr`.
- Static Linux amd64 binary cross-compiled at `dist/ibkr-linux-amd64`; not run on Linux.
- Installer shell syntax checked; installer and GitHub workflows were not executed remotely.
- All 89 recorded Rust source hashes remained unchanged.
- No actual credential files were copied.

The first race check found unsynchronized request-record access in the mock server;
its accessors now use the same mutex as the server handler, and the final race run passed.

No live IBKR requests, real orders, actual Redis server connections, or personal PostgreSQL
connections were made. Redis tests use an emulator. Database tests validate SQL and parameters
through a recorder, not a PostgreSQL engine. No release or deployment was performed.
