# IBKR Go CLI

Go port of the existing `ibkr-rs` OAuth-only REST implementation. The executable is named `ibkr`.
The Go CLI supports scripted REST commands and interactive utilities, with optional database persistence.
The common/admin command split is intentionally deferred until parity is established.

## Build and check

```sh
go build -o bin/ibkr ./cmd/ibkr
./bin/ibkr --help
go test -race ./...
go vet ./...
```

Use Go 1.25 or newer. OpenSSL is needed only for `oauth generate-materials`;
normal authentication uses Go's RSA, HMAC, PEM, ASN.1, and big-integer implementations.
No Rust executable or runtime is used.

## Layout

- `ibkr/`: reusable OAuth, session cache, REST endpoints, and order/market-data models.
- `internal/cli/`: command arguments, output, warning replies, interactive VWAP utility.
- `internal/config/`: dotenv loading and environment configuration.
- `internal/database/`: optional personal warehouse queries, isolated from the REST package.
- `internal/materials/`: OpenSSL key generation.
- `cmd/ibkr/`: executable entry point.

See [migration notes](docs/MIGRATION.md) for parity scope and deliberate differences.


`ibkr` is an OAuth-only IBKR CLI for Airflow tasks. Airflow invokes one command, reads exit status, and handles scheduling or retries.

Success writes JSON to stdout and exits `0`. Failures write diagnostics to stderr and exit nonzero.
Most commands leave persistence to Airflow: use stdout or `--output`, then let the DAG write results to storage. `--database` explicitly enables database integration: `fetch-history` writes bars to `warehouse.ibkr_bars`, and `stock-conid` uses `warehouse.conids` for lookup/storage. Setting `IBKR_DATABASE` alone never enables either behavior.

## Explicit Non-Goals

- No Client Portal Gateway.
- No HTTP job API.
- No Redis queue. Redis is used only as an optional OAuth Live Session Token cache.
- No Kafka.
- No WebSocket streaming.
- No watchlist, scanner, or broad contract lookup modules. Stock symbol-to-conid lookup is supported.
- No automatic fallback auth path.

## Example

For local development, copy `.env.example` to `.env` and fill in the IBKR values. Select that file explicitly with `--env-file .env`, or import it using `ibkr configure`. On a server, you can also provide the same `IBKR_*` variables through the process environment or your scheduler's secret manager.

### 1. Generate OAuth materials

This creates the local OpenSSL materials required by the IBKR OAuth setup. Send only `public_signature.pem`, `public_encryption.pem`, and `dhparam.pem` to IBKR. Never send or commit the generated private key files.

```bash
# Local Development
go run ./cmd/ibkr oauth generate-materials --out-dir ./secrets/ibkr-oauth

# Server Usage
ibkr oauth generate-materials --out-dir ./secrets/ibkr-oauth
```

Use `--force` only when you intentionally want to replace existing files:

```bash
# Local Development
go run ./cmd/ibkr oauth generate-materials --out-dir ./secrets/ibkr-oauth --force

# Server Usage
ibkr oauth generate-materials --out-dir ./secrets/ibkr-oauth --force
```

### 2. Initialize the brokerage session

This asks IBKR to initialize the authenticated brokerage session before protected account, order, or market-data calls. It uses the configured OAuth credentials and writes IBKR's JSON response to stdout.

```bash
# Local Development
go run ./cmd/ibkr init-session

# Server Usage
ibkr init-session
```

### 3. Look up a stock conid

By default this calls IBKR's stock lookup endpoint directly. With `--database`, it first looks up an active row in `warehouse.conids`, then falls back to IBKR when the symbol is not cached or the database is unavailable and attempts to store the resolved contract. It returns the selected symbol, English name, conid, and exchange. If multiple contracts match, the command fails so the caller can provide a more specific `--exchange` or filter choice.

```bash
# Local Development
go run ./cmd/ibkr stock-conid --symbol AAPL --exchange NASDAQ

# Server Usage
ibkr stock-conid --symbol AAPL --exchange NASDAQ
```

### 4. Fetch historical bars

This requests historical market-data bars for a known IBKR conid. By default the JSON response goes to stdout; use `--output` when Airflow should hand a file to a downstream task. Only with `--database` and a configured, connectable `IBKR_DATABASE` are returned bars also upserted into `warehouse.ibkr_bars`; database persistence is best-effort and does not change the JSON output.

```bash
# Local Development
go run ./cmd/ibkr fetch-history --conid 265598 --period 1d --bar 1min

# Server Usage
ibkr fetch-history --conid 265598 --period 1d --bar 1min --output /tmp/ibkr-history.json
```

#### Interactive history and explicit dates

Run `ibkr fetch-history` to enter the contract ID, dates, bar interval, end-date
inclusion, and timezone. Provided flags are retained; only missing required
values are prompted for. A fully specified command runs without prompts.

```sh
ibkr fetch-history
ibkr fetch-history --conid 265598 --bar 1d
ibkr fetch-history --conid 265598 --start-date 2026-09-01 --end-date 2026-09-07 --bar 1d
ibkr fetch-history --conid 265598 --start-date 2026-09-01 --end-date 2026-09-07 --inclusive=false --timezone America/New_York --bar 5min --output history.json
```

`--start_date` and `--end_date` are also accepted. Dates use `YYYY-MM-DD`.
The start date is always included; `--inclusive` defaults to `true` and controls
only the end date. The default timezone is UTC. Use `--timezone America/New_York`
for US local dates; daylight-saving transitions are respected. Inclusion is based
on each returned bar's timestamp, not the entire duration spanned by a weekly/monthly bar.
An empty or reversed range is rejected. Date flags cannot be combined with
`--period` or `--start-time`; existing period requests retain the raw API response.

Date-range requests run sequentially in bounded windows, merge and sort bars by
timestamp, remove duplicates, and exclude timestamps outside the requested range.
Their JSON includes `data`, `range`, `conid`, `requestCount`, and available bar scale
metadata. They do not reuse window-specific highs, lows, or counts as aggregate metadata.
A failed request or potentially truncated response aborts before output or database writes.
Empty market sessions remain empty; the CLI does not synthesize missing bars.
IBKR permissions and historical availability still apply. This behavior is covered
by local mock tests; it has not been validated against live IBKR.

Reference: [IBKR historical OHLC parameters](https://www.interactivebrokers.com/docs/web-api/api-reference/trading/trading-market-data/get-md-history).

## 5. Place an order

Order placement is non-interactive. The order input contains one order object or an array of order objects, passed inline with `--orders-json`. If IBKR returns warning prompts, the answers input must explicitly accept them by message substring or message id. Answers are layered from built-in defaults, the optional `IBKR_ORDERS_ANSWER_JSON` file path, optional `--answers-file`, and optional `--answers-json`; later layers override duplicate keys.
Run `init-session` before `order algos`, `order place`, `order modify`, or other protected IServer order calls.

Normal orders omit IB Algo fields:

```json
{
  "conid": 265598,
  "side": "BUY",
  "quantity": 1,
  "order_type": "MKT",
  "acct_id": "DU123456",
  "coid": "example-20260503-0001"
}
```

```json
{
  "price exceeds the Percentage constraint": true,
  "o354": true
}
```

```bash
ibkr order place --account-id DU123456 --orders-json '{"conid":265598,"side":"BUY","quantity":1,"order_type":"MKT","acct_id":"DU123456","coid":"example-20260503-0001"}' --answers-file ./answers.json
```

The same request with inline answers:

```bash
ibkr order place --account-id DU123456 --orders-json '{"conid":265598,"side":"BUY","quantity":1,"order_type":"MKT","acct_id":"DU123456","coid":"example-20260503-0001"}' --answers-json '{"o354":true}'
```

To inspect IB Algo strategies available for a contract, query the Web API algo endpoint after `init-session`.
Specify up to 8 case-sensitive algo ids with repeated `--algo` flags.

```bash
ibkr order algos --conid 265598 --algo Adaptive --algo Vwap --add-description --add-params --pretty
```

### Trade executions

Use `trades` to retrieve recent trade executions. This is execution history, not market-data history; use `fetch-history` for historical bars. IBKR supports up to 7 days for this endpoint and advises calling it once per session.

`brokerage-accounts` calls `/iserver/accounts`; this is distinct from the
`accounts` command, which calls `/portfolio/accounts`. IBKR may return an empty
trade list until brokerage account context has been loaded for the session. For
scripts and Airflow jobs, keep the warm-up sequence explicit:

```bash
ibkr init-session
ibkr brokerage-accounts
ibkr trades
sleep 5
ibkr trades --account-id DU123456 --days 7 --pretty
```

The five-second delay is orchestration policy rather than hidden CLI behavior.
Portfolio endpoints have their own preflight: call `accounts` before
`portfolio-summary`, `ledger`, `positions`, or `positions-live` for an
individual account. For the IServer `account-summary` command, use
`brokerage-accounts` as the account-context preflight.

### Account P&L and near-real-time positions

```bash
ibkr brokerage-accounts
ibkr account-pnl --pretty

ibkr accounts
ibkr positions-live --account-id DU123456 --pretty
```

`positions-live` uses the uncached REST endpoint and supports optional `--model`,
`--sort`, and `--direction a|d` filters. It does not open a WebSocket.

### Interactive quick VWAP order

`vwap-order` is an operator utility rather than an Airflow command. It is
listed separately at the bottom of `ibkr --help`. Missing values are prompted;
flags can prefill common values. The utility resolves the ticker directly with
IBKR, displays the complete payload, defaults submission confirmation to **no**,
and prompts separately for every warning returned by IBKR.

```bash
ibkr --env-file /secure/path/ibkr.env init-session
ibkr --env-file /secure/path/ibkr.env brokerage-accounts
ibkr --env-file /secure/path/ibkr.env vwap-order
```

The fixed order fields are `orderType=LMT`, `tif=DAY`, and `strategy=Vwap`.
Start and end times default to `15:30:00 US/Eastern` and
`16:00:00 US/Eastern`; they remain editable. `IBKR_ACCOUNT_ID` and
`IBKR_QUICK_ORDER_PREFIX` provide optional prompt defaults. This utility does
not connect to Postgres and does not persist the order locally. The brokerage
preflight remains a separate command so its response and any failure stay
visible.

Algo orders use the same order placement command with `strategy` and `strategy_parameters` in the order JSON:

```json
{
  "conid": 265598,
  "side": "BUY",
  "quantity": 100,
  "order_type": "LMT",
  "price": 185.5,
  "acct_id": "DU123456",
  "tif": "DAY",
  "strategy": "Vwap",
  "strategy_parameters": {
    "maxPctVol": 0.1,
    "startTime": "09:30:00 EST",
    "endTime": "15:30:00 EST",
    "allowPastEndTime": true
  }
}
```

## Interactive input and help

Help groups commands by Setup, Session, Accounts, Positions & Trades, Market Data,
Orders, and Help. Existing command names and flags are unchanged.

Terminal prompts display the label/default above a separate input field. Backspace and
Delete only edit that field; Unicode input, cursor keys, long lines, and terminal resizing
are supported. Enter accepts the input (or keeps the displayed default when empty).
Ctrl+C or Esc cancels; Ctrl+D cancels an empty field. Cancellation restores the terminal
and exits with status 130. Piped input continues to use newline-delimited values.

## Configuration

Run `ibkr configure` to import your existing credentials and files interactively:

```sh
ibkr configure
# Existing .env path: /secure/path/.env
# dhparam.pem path: Enter to keep the imported path
# private_encryption.pem path: Enter to keep the imported path
# private_signature.pem path: Enter to keep the imported path
# order_answers.json path: Enter to keep the imported path
ibkr init-session
```

Tokens are read from the selected `.env` and are never printed or requested again.
Missing required credentials must be filled in the source `.env` before importing.
Relative material paths are resolved against the source `.env` directory; `~/` is supported.
All four files are copied into a private `materials-*` directory under `~/.config/ibkr/`,
and the saved `~/.config/ibkr/.env` references their absolute paths. Only recognized IBKR
settings are imported. RSA keys, DH parameters, and the order-answer JSON object are
validated locally before activating the configuration; no IBKR request is made.
`order_answers.json` controls order confirmations, not OAuth authentication.
Files use mode `0600`, and newly created configuration/material directories use `0700`.
Re-running `configure` defaults to the saved configuration; Enter keeps each file path.
Failed setup leaves the active configuration intact. Previous material generations are
retained so an already-running process can finish using them.

By default, `ibkr` reads only `~/.config/ibkr/.env`. It never searches the current directory
or any parent for `.env`. Existing process environment variables still override file values,
including empty values, preserving Airflow environment-based configuration.
To select a different file, use `--env-file`; it replaces the default file rather than merging it.
Use this flag on `configure` to select a custom save destination as well.

```sh
ibkr --env-file /secure/path/ibkr.env init-session
```

To inspect the effective supported `IBKR_*` variables without validating credentials or calling IBKR, run:

```sh
ibkr --env-file /secure/path/ibkr.env env
```

Secret values are fully redacted in the output.

```text
IBKR_BASE_URL=https://api.ibkr.com/v1/api
IBKR_CONSUMER_KEY=<from IBKR>
IBKR_REALM=limited_poa
IBKR_ACCESS_TOKEN=<from IBKR self-service portal>
IBKR_ACCESS_TOKEN_SECRET=<from IBKR self-service portal>
IBKR_SIGNATURE_KEY_PATH=/secure/path/private_signature.pem
IBKR_ENCRYPTION_KEY_PATH=/secure/path/private_encryption.pem
IBKR_DH_PARAM_PATH=/secure/path/dhparam.pem
IBKR_TIMEOUT_SECONDS=30
IBKR_ACCOUNT_ID=DU123456
IBKR_QUICK_ORDER_PREFIX=kappa-k1
IBKR_DATABASE=postgres://user:password@host:5432/dbname
IBKR_LST_CACHE_MODE=redis
IBKR_REDIS_URL=redis://user:password@redis.example.internal:6379/0
IBKR_REDIS_KEY_PREFIX=ibkr:oauth:lst
IBKR_LST_REFRESH_SKEW_SECONDS=60
IBKR_LST_LOCK_TTL_SECONDS=15
```

## Live Session Token Cache

`ibkr` routes all Live Session Token lookup through one OAuth cache provider. Cache modes are:

- `redis`: share validated LSTs across short-lived CLI processes. If Redis is unavailable, the command fails instead of silently requesting an uncached token.
- `memory`: reuse the LST only inside the current CLI process. This is the default.
- `disabled`: request a fresh LST for each protected REST call.

Redis stores the validated LST payload with a TTL ending before IBKR expiry. The cache key uses hashed fingerprints of the base URL, consumer key, access token, and realm, and never stores raw identifiers in the key. Treat Redis as a secrets-adjacent system: use ACLs, private networking or TLS, restricted database access, and avoid broad shared Redis instances.

## Airflow Example

```python
BashOperator(
    task_id="lookup_aapl_conid",
    bash_command="ibkr stock-conid --symbol AAPL --output /tmp/aapl-conid.json",
    env={
        "IBKR_BASE_URL": "{{ var.value.ibkr_base_url }}",
        "IBKR_CONSUMER_KEY": "{{ var.value.ibkr_consumer_key }}",
        "IBKR_REALM": "limited_poa",
        "IBKR_ACCESS_TOKEN": "{{ var.value.ibkr_access_token }}",
        "IBKR_ACCESS_TOKEN_SECRET": "{{ var.value.ibkr_access_token_secret }}",
        "IBKR_SIGNATURE_KEY_PATH": "/opt/airflow/secrets/private_signature.pem",
        "IBKR_ENCRYPTION_KEY_PATH": "/opt/airflow/secrets/private_encryption.pem",
        "IBKR_DH_PARAM_PATH": "/opt/airflow/secrets/dhparam.pem",
        "IBKR_LST_CACHE_MODE": "redis",
        "IBKR_REDIS_URL": "{{ var.value.ibkr_redis_url }}",
    },
)
```

```python
BashOperator(
    task_id="fetch_aapl_history",
    bash_command=(
        "ibkr fetch-history --conid 265598 --period 1d --bar 1min "
        "--output /tmp/ibkr-history.json"
    ),
    env={
        "IBKR_BASE_URL": "{{ var.value.ibkr_base_url }}",
        "IBKR_CONSUMER_KEY": "{{ var.value.ibkr_consumer_key }}",
        "IBKR_REALM": "limited_poa",
        "IBKR_ACCESS_TOKEN": "{{ var.value.ibkr_access_token }}",
        "IBKR_ACCESS_TOKEN_SECRET": "{{ var.value.ibkr_access_token_secret }}",
        "IBKR_SIGNATURE_KEY_PATH": "/opt/airflow/secrets/private_signature.pem",
        "IBKR_ENCRYPTION_KEY_PATH": "/opt/airflow/secrets/private_encryption.pem",
        "IBKR_DH_PARAM_PATH": "/opt/airflow/secrets/dhparam.pem",
        "IBKR_LST_CACHE_MODE": "redis",
        "IBKR_REDIS_URL": "{{ var.value.ibkr_redis_url }}",
    },
)
```

Airflow should treat exit code `0` as success and any nonzero exit code as task failure.
Run `init-session` before protected IBKR calls. If IBKR returns `Bad Request: no bridge`, run `init-session` again; Redis caches OAuth Live Session Tokens, not brokerage bridge state.
For long-running jobs, call `tickle` about once per minute to keep an already-initialized brokerage session alive.

## API Surface

Implemented REST/CLI commands:

- `auth-status`: `iserver/auth/status`
- `init-session`: `iserver/auth/ssodh/init`
- `tickle`: `tickle`
- `fetch-history`: `iserver/marketdata/history`
- `stock-conid`: `trsrv/stocks`
- `accounts`: `portfolio/accounts`
- `brokerage-accounts`: `iserver/accounts`
- `account-pnl`: `iserver/account/pnl/partitioned`
- `account-summary`: `iserver/account/{account_id}/summary`
- `portfolio-summary`: `portfolio/{account_id}/summary`
- `ledger`: `portfolio/{account_id}/ledger`
- `positions`: `portfolio/{account_id}/positions/{page}`
- `positions-live`: `portfolio2/{account_id}/positions`
- `trades`: `iserver/account/trades/`
- `live-orders`: `iserver/account/orders`
- `order algos`: `iserver/contract/{conid}/algos`
- `order place`: `iserver/account/{account_id}/orders`
- `order whatif`: `iserver/account/{account_id}/orders/whatif`
- `order reply`: `iserver/reply/{reply_id}`
- `order cancel`: `iserver/account/{account_id}/order/{order_id}`
- `order modify`: `iserver/account/{account_id}/order/{order_id}`
- `order status`: `iserver/account/order/status/{order_id}`
- `vwap-order`: interactive `trsrv/stocks` lookup followed by one VWAP
  order submission

Missing but relevant functions include typed response models for accounts,
positions, trades, live orders, auth, and session calls.

Historical data, stock conid lookup, and order placement have non-trivial typed request shapes today. The other implemented endpoints are still thin JSON passthroughs with little or no request structure.

## Local fee-plan calculation

```sh
ibkr order fee-plan --orders-json '{"quantity":100,"price":100}' --pretty
```

This preserves the Rust implementation's local 10,000 notional threshold heuristic.
It does not query fees or change an IBKR account pricing plan.

## Release installation

Releases are published to [SKKUGoon/cli-ibkr-go](https://github.com/SKKUGoon/cli-ibkr-go/releases).
The included workflow packages Linux amd64 and macOS arm64 when a version tag
is pushed to that repository. The current release version is `3.0.1` (`v3.0.1` tag).
After that release is published, install with:

```sh
./deploy-ibkr.sh v3.0.1
```

The installer checks SHA-256 sums and installs `ibkr` into `~/.local/bin` by default.
Override the destination with `IBKR_INSTALL_DIR`. Downloads always come from
`SKKUGoon/cli-ibkr-go`. An existing `ibkr` at the installation destination is replaced.

To publish from a committed checkout connected to this repository:

```sh
git tag v3.0.1
git push origin v3.0.1
```

The workflow builds the binaries and creates the GitHub release with SHA-256 checksums.
Live IBKR and production database connectivity have not yet been validated.

## Explicit database access

All commands default to output without PostgreSQL lookup or persistence, even if
`IBKR_DATABASE` is configured. Add the global `--database` flag to enable existing
database functionality in `fetch-history` and `stock-conid`. It does not add database
functionality to other commands. Output still goes to stdout or `--output`.

```sh
ibkr fetch-history --conid 265598 --period 1d --bar 1min
ibkr fetch-history --conid 265598 --period 1d --bar 1min --database
ibkr stock-conid --symbol AAPL --database
```

The same default applies to interactive commands. `--database=false` disables DB
access explicitly. When enabled, missing/unavailable database configuration retains
the existing best-effort behavior with warnings on stderr. OAuth Redis token caching
is separate and continues to use its existing cache settings.

See [CHANGELOG.md](CHANGELOG.md) for release changes and [docs/VERIFICATION.md](docs/VERIFICATION.md) for validation scope.
