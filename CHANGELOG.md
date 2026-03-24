# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- Initial CustomerStore interface and SQLCustomerStore implementation
- AES-256-GCM PII encryption with KeyProvider interface
- SHA-256 NI hash for dedup lookup
- FixedKeyProvider and EnvKeyProvider implementations
- Full test suite using pglike in-memory DB
