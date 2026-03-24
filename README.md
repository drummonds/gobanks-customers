# gobanks-customers

Customer identity and PII storage for gobank. Companion library to [go-luca](https://github.com/drummonds/go-luca).

## Design

- Accepts a `*sql.DB` (shares the same database as go-luca)
- Creates its own tables with `cust_` prefix (no clashes with go-luca)
- PII encrypted at rest with AES-256-GCM via a `KeyProvider` interface
- NI number hashed (SHA-256) for dedup lookup without decryption

## Usage

```go
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
