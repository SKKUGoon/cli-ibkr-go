# Changelog

## 3.0.1 — 2026-09-09

- Group help by setup, session, accounts, positions/trades, market data, orders, and help.
- Fix blocked Ctrl+C input cancellation; support Esc and empty-input Ctrl+D.
- Protect prompt labels with a separate terminal input editor supporting Unicode, cursor
  movement, Backspace/Delete, long input, and resizing. Restore terminal state on exit.
- Add real PTY regression checks for editing, cancellation, and terminal restoration.

## 3.0.0 — 2026-09-09

### Breaking changes

- Stop searching the working directory and parents for `.env`. The default is now
  `~/.config/ibkr/.env`. Import existing settings with `ibkr configure`, or select a file
  explicitly with `--env-file`. Process environment variables retain their precedence.

### Added

- Interactive `configure` imports credentials from an existing `.env`, then prompts for
  three PEM paths and `order_answers.json`, retaining supplied paths on Enter.
- Validate and copy selected files into private configuration storage before atomically
  activating the new `.env`. Failed setup preserves the active configuration.
- Re-running setup uses saved settings; `--env-file` supports a custom save destination.
- Credentials are never printed; malformed dotenv errors do not echo file contents.

## 2.0.0 — 2026-09-09

### Breaking changes

- Rename `quick-vwap-order` to `vwap-order`. Update existing scripts; the former name is removed.
- PostgreSQL lookup and storage now require the global `--database` flag. Setting
  `IBKR_DATABASE` alone no longer enables history storage or stock contract lookup/storage.
  Add `--database` to scripts that depend on those behaviors. JSON output is retained.

### Added

- Interactive `fetch-history`, preserving supplied flags and existing `--period` requests.
- `--start-date` / `--end-date` (also underscore spellings), `--inclusive=true` by default,
  and `--timezone` (UTC by default).
- Bounded date-range requests with filtering, sorting and deduplication.
- Separate interactive utilities in help output.
- Installer downloads from the fixed official repository.

## 1.0.0 — 2026-09-08

- Initial Go port of the Rust OAuth REST CLI, including key generation, orders,
  market data, optional PostgreSQL integration and Redis token caching.
- Linux amd64 and macOS arm64 release archives with SHA-256 checksums.
