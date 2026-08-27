package customers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// dbtx is the subset of database operations the store uses, satisfied by
// both *sql.DB and *sql.Tx so a store can run inside a caller's transaction.
type dbtx interface {
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// txBeginner is satisfied by *sql.DB but not *sql.Tx; see begin.
type txBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// SQLCustomerStore implements CustomerStore backed by SQL.
type SQLCustomerStore struct {
	db  dbtx
	key KeyProvider
}

// WithTx returns a store view that runs every statement on tx. The caller
// owns the transaction: methods that normally manage their own transaction
// run directly on tx instead, nothing is committed by the store, and on
// error the caller should roll tx back.
func (s *SQLCustomerStore) WithTx(tx *sql.Tx) *SQLCustomerStore {
	return &SQLCustomerStore{db: tx, key: s.key}
}

// begin starts a transaction when the store owns a *sql.DB. When the store
// is bound to a caller's transaction (WithTx), it returns that transaction
// with no-op commit and rollback — the caller owns the transaction's fate.
func (s *SQLCustomerStore) begin(ctx context.Context) (q dbtx, commit func() error, rollback func() error, err error) {
	if b, ok := s.db.(txBeginner); ok {
		tx, err := b.BeginTx(ctx, nil)
		if err != nil {
			return nil, nil, nil, err
		}
		return tx, tx.Commit, tx.Rollback, nil
	}
	noop := func() error { return nil }
	return s.db, noop, noop, nil
}

// NewSQLCustomerStore wraps a pre-opened *sql.DB and ensures the schema exists.
func NewSQLCustomerStore(db *sql.DB, key KeyProvider) (*SQLCustomerStore, error) {
	s := &SQLCustomerStore{db: db, key: key}
	if err := s.createSchema(); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return s, nil
}

func (s *SQLCustomerStore) createSchema() error {
	// Execute each statement separately for pglike compatibility.
	for stmt := range strings.SplitSeq(SchemaSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:min(len(stmt), 60)], err)
		}
	}
	return nil
}

func (s *SQLCustomerStore) Create(ctx context.Context, cust CustomerRecord, pii PIIInput) error {
	key, err := s.key.PIIKey()
	if err != nil {
		return fmt.Errorf("get key: %w", err)
	}

	if cust.ID == "" {
		cust.ID = uuid.New().String()
	}

	encName, err := encrypt(key, pii.Name)
	if err != nil {
		return fmt.Errorf("encrypt name: %w", err)
	}
	encNI, err := encrypt(key, pii.NI)
	if err != nil {
		return fmt.Errorf("encrypt ni: %w", err)
	}
	encDOB, err := encrypt(key, pii.DOB)
	if err != nil {
		return fmt.Errorf("encrypt dob: %w", err)
	}
	encAddr, err := encrypt(key, pii.Address)
	if err != nil {
		return fmt.Errorf("encrypt address: %w", err)
	}
	encEmail, err := encrypt(key, pii.Email)
	if err != nil {
		return fmt.Errorf("encrypt email: %w", err)
	}
	encPhone, err := encrypt(key, pii.Phone)
	if err != nil {
		return fmt.Errorf("encrypt phone: %w", err)
	}
	niHash := hashNI(pii.NI)

	tx, commit, rollback, err := s.begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO cust_customers (id, ref, join_date, kyc_verified, kyc_last_check, kyc_risk_rating)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		cust.ID, cust.Ref, cust.JoinDate, cust.KYCVerified, cust.KYCLastCheck, cust.KYCRiskRating,
	)
	if err != nil {
		return fmt.Errorf("insert customer: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO cust_pii (customer_id, encrypted_name, encrypted_ni, encrypted_dob, encrypted_address, encrypted_email, encrypted_phone, ni_hash, key_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		cust.ID, encName, encNI, encDOB, encAddr, encEmail, encPhone, niHash, s.key.CurrentKeyVersion(),
	)
	if err != nil {
		return fmt.Errorf("insert pii: %w", err)
	}

	return commit()
}

func (s *SQLCustomerStore) Get(ctx context.Context, ref string) (*CustomerRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, ref, join_date, kyc_verified, kyc_last_check, kyc_risk_rating FROM cust_customers WHERE ref = $1`, ref)
	return s.scanCustomer(row)
}

// GetByID retrieves a customer by their primary key ID.
func (s *SQLCustomerStore) GetByID(ctx context.Context, id string) (*CustomerRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, ref, join_date, kyc_verified, kyc_last_check, kyc_risk_rating FROM cust_customers WHERE id = $1`, id)
	return s.scanCustomer(row)
}

func (s *SQLCustomerStore) scanCustomer(row *sql.Row) (*CustomerRecord, error) {
	var c CustomerRecord
	var joinDate, kycLastCheck string
	err := row.Scan(&c.ID, &c.Ref, &joinDate, &c.KYCVerified, &kycLastCheck, &c.KYCRiskRating)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan customer: %w", err)
	}
	c.JoinDate = parseTime(joinDate)
	c.KYCLastCheck = parseTime(kycLastCheck)
	return &c, nil
}

func (s *SQLCustomerStore) GetPII(ctx context.Context, ref string) (*PIIData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT p.encrypted_name, p.encrypted_ni, p.encrypted_dob, p.encrypted_address, p.encrypted_email, p.encrypted_phone, p.key_version
		 FROM cust_pii p JOIN cust_customers c ON p.customer_id = c.id WHERE c.ref = $1`, ref)
	return s.decryptPIIRow(row)
}

// GetPIIByID retrieves PII by customer primary key ID.
func (s *SQLCustomerStore) GetPIIByID(ctx context.Context, id string) (*PIIData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT encrypted_name, encrypted_ni, encrypted_dob, encrypted_address, encrypted_email, encrypted_phone, key_version
		 FROM cust_pii WHERE customer_id = $1`, id)
	return s.decryptPIIRow(row)
}

func (s *SQLCustomerStore) decryptPIIRow(row *sql.Row) (*PIIData, error) {
	var encName, encNI, encDOB, encAddr, encEmail, encPhone string
	var keyVersion int
	err := row.Scan(&encName, &encNI, &encDOB, &encAddr, &encEmail, &encPhone, &keyVersion)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan pii: %w", err)
	}

	key, err := s.key.PIIKeyByVersion(keyVersion)
	if err != nil {
		return nil, fmt.Errorf("get key version %d: %w", keyVersion, err)
	}

	name, err := decrypt(key, encName)
	if err != nil {
		return nil, fmt.Errorf("decrypt name: %w", err)
	}
	ni, err := decrypt(key, encNI)
	if err != nil {
		return nil, fmt.Errorf("decrypt ni: %w", err)
	}
	dob, err := decrypt(key, encDOB)
	if err != nil {
		return nil, fmt.Errorf("decrypt dob: %w", err)
	}
	addr, err := decrypt(key, encAddr)
	if err != nil {
		return nil, fmt.Errorf("decrypt address: %w", err)
	}
	email, err := decrypt(key, encEmail)
	if err != nil {
		return nil, fmt.Errorf("decrypt email: %w", err)
	}
	phone, err := decrypt(key, encPhone)
	if err != nil {
		return nil, fmt.Errorf("decrypt phone: %w", err)
	}

	return &PIIData{
		Name:    name,
		NI:      ni,
		DOB:     dob,
		Address: addr,
		Email:   email,
		Phone:   phone,
	}, nil
}

func (s *SQLCustomerStore) GetName(ctx context.Context, ref string) (string, error) {
	pii, err := s.GetPII(ctx, ref)
	if err != nil {
		return "", err
	}
	if pii == nil {
		return ref, nil // fallback
	}
	return pii.Name, nil
}

// GetNameByID returns the decrypted name for a customer by primary key ID.
func (s *SQLCustomerStore) GetNameByID(ctx context.Context, id string) (string, error) {
	pii, err := s.GetPIIByID(ctx, id)
	if err != nil {
		return "", err
	}
	if pii == nil {
		return id, nil // fallback
	}
	return pii.Name, nil
}

func (s *SQLCustomerStore) List(ctx context.Context, offset, limit int) ([]CustomerRecord, int, error) {
	total, err := s.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, ref, join_date, kyc_verified, kyc_last_check, kyc_risk_rating
		 FROM cust_customers ORDER BY created_at LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list customers: %w", err)
	}
	defer rows.Close()

	var result []CustomerRecord
	for rows.Next() {
		var c CustomerRecord
		var joinDate, kycLastCheck string
		if err := rows.Scan(&c.ID, &c.Ref, &joinDate, &c.KYCVerified, &kycLastCheck, &c.KYCRiskRating); err != nil {
			return nil, 0, fmt.Errorf("scan row: %w", err)
		}
		c.JoinDate = parseTime(joinDate)
		c.KYCLastCheck = parseTime(kycLastCheck)
		result = append(result, c)
	}
	return result, total, rows.Err()
}

func (s *SQLCustomerStore) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cust_customers`).Scan(&n)
	return n, err
}

func (s *SQLCustomerStore) Reset(ctx context.Context) error {
	// Delete PII first (FK constraint)
	if _, err := s.db.ExecContext(ctx, `DELETE FROM cust_pii`); err != nil {
		return fmt.Errorf("delete pii: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM cust_customers`); err != nil {
		return fmt.Errorf("delete customers: %w", err)
	}
	return nil
}

// RotateKeys re-encrypts all PII rows that are not on the current key version.
// Returns the number of rows rotated.
func (s *SQLCustomerStore) RotateKeys(ctx context.Context) (int, error) {
	currentVersion := s.key.CurrentKeyVersion()
	newKey, err := s.key.PIIKey()
	if err != nil {
		return 0, fmt.Errorf("get current key: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT customer_id, encrypted_name, encrypted_ni, encrypted_dob, encrypted_address, encrypted_email, encrypted_phone, key_version
		 FROM cust_pii WHERE key_version != $1`, currentVersion)
	if err != nil {
		return 0, fmt.Errorf("select stale rows: %w", err)
	}
	defer rows.Close()

	type piiRow struct {
		customerID                                       string
		encName, encNI, encDOB, encAddr, encEmail, encPh string
		keyVersion                                       int
	}
	var stale []piiRow
	for rows.Next() {
		var r piiRow
		if err := rows.Scan(&r.customerID, &r.encName, &r.encNI, &r.encDOB, &r.encAddr, &r.encEmail, &r.encPh, &r.keyVersion); err != nil {
			return 0, fmt.Errorf("scan row: %w", err)
		}
		stale = append(stale, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, r := range stale {
		oldKey, err := s.key.PIIKeyByVersion(r.keyVersion)
		if err != nil {
			return 0, fmt.Errorf("get key version %d: %w", r.keyVersion, err)
		}

		// Decrypt with old key
		name, err := decrypt(oldKey, r.encName)
		if err != nil {
			return 0, fmt.Errorf("decrypt name for %s: %w", r.customerID, err)
		}
		ni, err := decrypt(oldKey, r.encNI)
		if err != nil {
			return 0, fmt.Errorf("decrypt ni for %s: %w", r.customerID, err)
		}
		dob, err := decrypt(oldKey, r.encDOB)
		if err != nil {
			return 0, fmt.Errorf("decrypt dob for %s: %w", r.customerID, err)
		}
		addr, err := decrypt(oldKey, r.encAddr)
		if err != nil {
			return 0, fmt.Errorf("decrypt address for %s: %w", r.customerID, err)
		}
		email, err := decrypt(oldKey, r.encEmail)
		if err != nil {
			return 0, fmt.Errorf("decrypt email for %s: %w", r.customerID, err)
		}
		phone, err := decrypt(oldKey, r.encPh)
		if err != nil {
			return 0, fmt.Errorf("decrypt phone for %s: %w", r.customerID, err)
		}

		// Re-encrypt with new key
		encName, err := encrypt(newKey, name)
		if err != nil {
			return 0, fmt.Errorf("re-encrypt name for %s: %w", r.customerID, err)
		}
		encNI, err := encrypt(newKey, ni)
		if err != nil {
			return 0, fmt.Errorf("re-encrypt ni for %s: %w", r.customerID, err)
		}
		encDOB, err := encrypt(newKey, dob)
		if err != nil {
			return 0, fmt.Errorf("re-encrypt dob for %s: %w", r.customerID, err)
		}
		encAddr, err := encrypt(newKey, addr)
		if err != nil {
			return 0, fmt.Errorf("re-encrypt address for %s: %w", r.customerID, err)
		}
		encEmail, err := encrypt(newKey, email)
		if err != nil {
			return 0, fmt.Errorf("re-encrypt email for %s: %w", r.customerID, err)
		}
		encPhone, err := encrypt(newKey, phone)
		if err != nil {
			return 0, fmt.Errorf("re-encrypt phone for %s: %w", r.customerID, err)
		}

		_, err = s.db.ExecContext(ctx,
			`UPDATE cust_pii SET encrypted_name=$1, encrypted_ni=$2, encrypted_dob=$3, encrypted_address=$4, encrypted_email=$5, encrypted_phone=$6, key_version=$7
			 WHERE customer_id=$8`,
			encName, encNI, encDOB, encAddr, encEmail, encPhone, currentVersion, r.customerID,
		)
		if err != nil {
			return 0, fmt.Errorf("update row %s: %w", r.customerID, err)
		}
	}

	return len(stale), nil
}

// parseTime handles both RFC3339 and SQLite datetime formats from pglike.
func parseTime(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t
	}
	t, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", s)
	return t
}
