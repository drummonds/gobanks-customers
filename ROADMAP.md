# Roadmap

## Done
- CustomerStore interface + SQL implementation
- PII encryption (AES-256-GCM)
- KeyProvider interface (Fixed, Env, Versioned)
- NI hash dedup
- Key rotation with versioned keys and RotateKeys()
- go-luca integration (shared DB, UUID v4 IDs)
- Module path migrated to git.bytestone.uk/hum3

## Next
- Batch name lookup for list pages (avoid N+1)
- Bitwarden/GCP Secrets Manager KeyProvider
- Customer search by name (encrypted search or materialized index)
