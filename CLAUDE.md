# gobanks-customers

Go library for encrypted customer PII storage. Companion to go-luca.

## Architecture
- Single package: `customers` (no cmd/, no internal/)
- SQL backend via pglike (codeberg.org/hum3/go-postgres) or PostgreSQL
- Tables prefixed `cust_` to share DB with go-luca without clashes
- Customer IDs are UUID v4, same as go-luca — link via `GetByID`/`GetPIIByID`

## Encryption
- AES-256-GCM for all PII fields
- Each row stores `key_version` for rotation support
- `KeyProvider` interface abstracts key source
- `RotateKeys()` re-encrypts stale rows to current key version

## Testing
- `task test` or `go test ./...`
- Tests use pglike in-memory SQLite (`:memory:`)
- Key rotation tests use `VersionedKeyProvider`

## Module
- `codeberg.org/hum3/gobanks-customers`
