# Rust to Go migration

## Scope

Source: sibling `ibkr-rs`, version 0.10.1. `RUST_SOURCE_SHA256.json` records the Rust
source-file hashes used for this port. The existing Rust checkout is not modified.
This is a Go implementation, not a wrapper or fallback to the Rust executable.

| Rust command | Go command | Behavior |
| --- | --- | --- |
| `ibkrctl oauth generate-materials` | `ibkr oauth generate-materials` | Seven OpenSSL steps; RSA private/public keys, PKCS#8 copies, DH parameters |
| `ibkrctl env` | `ibkr env` | Effective supported configuration, without requiring authentication |
| `auth-status`, `init-session`, `tickle` | Same subcommands | OAuth-protected session operations |
| `accounts`, `brokerage-accounts`, `account-pnl` | Same subcommands | Distinct portfolio and brokerage endpoints |
| `account-summary`, `portfolio-summary`, `ledger` | Same subcommands | Account-specific REST calls |
| `positions`, `positions-live` | Same subcommands | Paged and uncached REST positions |
| `trades`, `live-orders` | Same subcommands | Account/date filters, refresh flag |
| `fetch-history` | Same subcommand | Raw JSON and optional best-effort warehouse bar upsert |
| `stock-conid` | Same subcommand | Optional active warehouse lookup, REST resolution, upsert |
| `order algos` | Same subcommand | Up to eight IDs, semicolon query, description/parameter flags |
| `order place`, `order modify` | Same subcommands | Alias normalization, validation, warning-answer loop |
| `order whatif`, `order reply`, `order cancel`, `order status` | Same subcommands | Same REST request semantics |
| `order fee-plan` | Same subcommand | Offline notional heuristic; no account pricing-plan change |
| `quick-vwap-order` | Same subcommand | Editable inputs, payload review, default-no confirmation, individual warning prompts |

Global flags: `--env-file`, `--output`, `--pretty`, `--timeout-seconds`, `--version` / `-V`.
Boolean value flags such as `--default-filtering false` and `--outside-rth false` accept
both separate values and `=false` syntax. Session initialization remains explicit.

## Configuration and credentials

The same `IBKR_*` variables are accepted. `.env` is found by searching upward from the
working directory; process variables override it. `--env-file` selects an explicit file.
PKCS#1 and PKCS#8 RSA private keys are accepted. RSA-SHA256 signs the token exchange;
HMAC-SHA256 signs protected requests after the DH/HMAC-SHA1 session derivation and proof check.

No real `.env`, PEM, PKCS#8, token, or server credentials are copied. Existing key files
can be referenced by absolute path in the new environment configuration; generating new
keys is not required to use an existing IBKR OAuth registration.

## Preserved application policies

- The built-in answer map accepts o163, o451, o354, and o10331 (plus their existing text
  matches). Overrides layer in this order: defaults, optional `IBKR_ORDERS_ANSWER_JSON`
  file, `--answers-file`, `--answers-json`. Message IDs take precedence over text.
- Unknown or rejected automatic warnings fail. The reply loop retains the Rust iteration limit.
- The fee-plan command preserves the source's 10,000 notional threshold calculation.
  It is a local heuristic, not a verified universal IBKR commission schedule.
- Stock cache lookup and history persistence retain the existing best-effort database behavior.
  Missing/unavailable DB connections are diagnosed on stderr. Ambiguous active contracts fail.
- Database schema is not created or migrated. Existing `warehouse.conids` and
  `warehouse.ibkr_bars` are required when database integration is configured.
- The SQL bar batch deduplicates by conid, timestamp and duration, keeping the last row.
- Redis uses the same SHA-256 fingerprint key layout and cached JSON fields as Rust.
  Redis failure does not trigger a direct OAuth fallback. Memory and disabled modes remain.

## Deliberate differences

- Binary name is `ibkr`; version output and User-Agent identify the Go implementation.
- `env` completely redacts secrets rather than exposing their first and last characters.
- Key generation occurs in a private staging directory. Generation failures do not replace
  existing keys; publishing refuses existing files unless `--force` is explicit.
- Redis lock release uses atomic compare-and-delete, avoiding the original GET/DEL race.
- HTTP uses the configured host, rejects redirects, and handles gzip through Go's transport.
- Empty credentials, invalid URLs/durations, and invalid DH peer values fail explicitly.
- Invalid dotenv input fails rather than being silently ignored.
- `--compete=false` and `--force=false` are supported; the source's true-default clap switches
  cannot express those values. Unknown commands fail with empty stdout.
- Interactive prompts use line input with visible editable defaults instead of dialoguer's
  terminal widgets. Side choices accept BUY/SELL. Prompt text and help formatting differ.
- Warning logging honors the common global and worker-module `IBKR_LOG` levels;
  Rust-specific tracing span-filter expressions are not implemented.
- Results written with `--output` use private permissions for newly created files.
- Go build/release files replace Cargo and Rust-target packaging. No releases are published.

## Validation commands

```sh
go test -race ./...
go vet ./...
go build -o bin/ibkr ./cmd/ibkr
IBKR_TEST_OPENSSL=1 go test ./internal/materials -run TestOpenSSLIntegration -count=1
```

Optional request/output comparison against an already-built Rust executable:

```sh
IBKR_RUST_REFERENCE_BINARY=/absolute/path/to/ibkrctl \
  go test ./internal/cli -run TestRESTCommandSurface -count=1 -v
```

The differential suite uses synthetic credentials and a loopback OAuth peer. It compares
HTTP methods, paths, query values, JSON request bodies and JSON output for compatible
commands. The two newly expressible false flags are tested in Go only.

The tests cover token crypto, memory/Redis caching, REST command routing, order warnings,
VWAP payloads and cancellation, output separation, dotenv precedence, key generation,
bar parsing/deduplication, and database query parameters. The SQL recorder is not a live
PostgreSQL engine. Live IBKR authentication, real order execution and the user's actual
DB schema/connectivity remain unverified.
