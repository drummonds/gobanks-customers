# gobanks-customers

Customer identity and PII storage for gobank. Companion library to [go-luca](https://codeberg.org/hum3/go-luca).

## Design

- Accepts a `*sql.DB` (shares the same database as go-luca)
- Creates its own tables with `cust_` prefix (no clashes with go-luca)
- PII encrypted at rest with AES-256-GCM via a `KeyProvider` interface
- NI number hashed (SHA-256) for dedup lookup without decryption
- Versioned encryption keys with `RotateKeys()` for key rotation
- Customer IDs are UUID v4 — same format as go-luca's `customers.id`

## Key Rotation

Each PII row stores the `key_version` used to encrypt it. To rotate:

1. Create a `VersionedKeyProvider` with both the old and new keys
2. Call `store.RotateKeys(ctx)` — re-encrypts all rows not on the current version

```go
vkp := customers.VersionedKeyProvider{
    Keys:    map[int][]byte{1: oldKey, 2: newKey},
    Current: 2,
}
store, _ := customers.NewSQLCustomerStore(db, vkp)
n, _ := store.RotateKeys(ctx)
fmt.Printf("rotated %d rows\n", n)
```

## go-luca Integration

Both libraries use UUID v4 customer IDs. Pass the same `*sql.DB` to both:

```go
ledger, _ := luca.NewSQLLedger(db)
custStore, _ := customers.NewSQLCustomerStore(db, keyProvider)

// go-luca creates customer, gobanks-customers stores PII under same ID
id, _ := ledger.CreateCustomer("Alice Smith")
custStore.Create(ctx, customers.CustomerRecord{ID: id, Ref: "cust-001", ...}, pii)

// Look up PII by go-luca customer ID
pii, _ := custStore.GetPIIByID(ctx, id)
```

## Usage

```go
import (
    "database/sql"
    customers "codeberg.org/hum3/gobanks-customers"
    _ "codeberg.org/hum3/go-postgres"
)

db, _ := sql.Open("pglike", ":memory:")
key := customers.FixedKeyProvider{Key: []byte("32-byte-key-here!!!!!!!!!!!!!!!")}
store, _ := customers.NewSQLCustomerStore(db, key)

store.Create(ctx, customers.CustomerRecord{
    ID: "uuid", Ref: "cust-001", JoinDate: time.Now(),
    KYCVerified: true, KYCLastCheck: time.Now(), KYCRiskRating: "Low",
}, customers.PIIInput{
    Name: "Alice Smith", NI: "AB123456C", DOB: "1990-01-01",
    Address: "10 High St", Email: "alice@example.com", Phone: "07123456789",
})
```

## Links

- Source: https://codeberg.org/hum3/gobanks-customers
- Mirror: https://github.com/drummonds/gobanks-customers
