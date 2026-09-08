# IBKR Go

This repository ports the OAuth-only Rust IBKR CLI into Go. The executable is `ibkr`.

- Use Go. Keep reusable OAuth/REST code in `ibkr/` independent of CLI and database code.
- Preserve the source command semantics during this first migration. Do not add WebSockets,
  Client Portal Gateway, job queues, or an HTTP job server.
- Keep named functions, explicit typed interfaces, small focused modules, and propagated errors.
- Common/admin command reorganization is a subsequent change. Optional database integration is
  currently confined to stock-conid and fetch-history, as in the Rust implementation.
- JSON results go to stdout (or --output); prompts and diagnostics go to stderr. `env` is text.
- Never copy real dotenv files, private keys, tokens, or database credentials into this repository.
- Tests must not contact live IBKR or the user's server. Use the local OAuth peer, Redis emulator,
  and database query recorder. Never run live order operations as validation.
- Run `gofmt`, `go vet ./...`, `go test -race ./...`, and `go build -o bin/ibkr ./cmd/ibkr` after changes.
- `IBKR_TEST_OPENSSL=1 go test ./internal/materials -run TestOpenSSLIntegration -count=1`
  generates and validates temporary test keys.
