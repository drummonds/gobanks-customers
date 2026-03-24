package customers

// SchemaSQL contains the DDL for customer tables.
// All table names use the cust_ prefix to avoid clashing with go-luca tables.
const SchemaSQL = `
CREATE TABLE IF NOT EXISTS cust_customers (
    id TEXT PRIMARY KEY,
    ref VARCHAR(20) UNIQUE NOT NULL,
    join_date TIMESTAMP NOT NULL,
    kyc_verified BOOLEAN NOT NULL DEFAULT true,
    kyc_last_check TIMESTAMP NOT NULL,
    kyc_risk_rating VARCHAR(20) NOT NULL DEFAULT 'Low',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_cust_customers_created_at ON cust_customers (created_at);
CREATE INDEX IF NOT EXISTS idx_cust_customers_ref ON cust_customers (ref);

CREATE TABLE IF NOT EXISTS cust_pii (
    customer_id TEXT PRIMARY KEY REFERENCES cust_customers(id),
    encrypted_name VARCHAR(500) NOT NULL,
    encrypted_ni VARCHAR(500) NOT NULL,
    encrypted_dob VARCHAR(500) NOT NULL,
    encrypted_address VARCHAR(500) NOT NULL,
    encrypted_email VARCHAR(500) NOT NULL,
    encrypted_phone VARCHAR(500) NOT NULL,
    ni_hash VARCHAR(64) NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_cust_pii_ni_hash ON cust_pii (ni_hash);
`
