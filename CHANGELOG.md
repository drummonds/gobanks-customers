# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Changed
- Module path migrated from `codeberg.org/hum3/gobanks-customers` to `git.bytestone.uk/hum3/gobanks-customers`
- go-postgres dependency moved to `git.bytestone.uk/hum3/go-postgres` and bumped to v0.5.5

## [0.1.0] - 2026-03-24

 - Merge branch 'task/WTbegin'

### Added
- Initial CustomerStore interface and SQLCustomerStore implementation
- AES-256-GCM PII encryption with KeyProvider interface
- SHA-256 NI hash for dedup lookup
- FixedKeyProvider, EnvKeyProvider, and VersionedKeyProvider implementations
- Key rotation via versioned encryption keys and `RotateKeys()` method
- `key_version` column in `cust_pii` table tracks which key encrypted each row
- go-luca integration: shared `*sql.DB`, compatible UUID v4 customer IDs
- Full test suite including key rotation and mixed-version tests
- Module path: `codeberg.org/hum3/gobanks-customers`
