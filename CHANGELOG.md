# Changelog

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
