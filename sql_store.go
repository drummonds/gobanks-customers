package customers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SQLCustomerStore implements CustomerStore backed by SQL.
type SQLCustomerStore struct {
	db  *sql.DB
	key KeyProvider
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
	for _, stmt := range strings.Split(SchemaSQL, ";") {
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO cust_customers (id, ref, join_date, kyc_verified, kyc_last_check, kyc_risk_rating)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		cust.ID, cust.Ref, cust.JoinDate, cust.KYCVerified, cust.KYCLastCheck, cust.KYCRiskRating,
	)
	if err != nil {
		return fmt.Errorf("insert customer: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO cust_pii (customer_id, encrypted_name, encrypted_ni, encrypted_dob, encrypted_address, encrypted_email, encrypted_phone, ni_hash)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		cust.ID, encName, encNI, encDOB, encAddr, encEmail, encPhone, niHash,
	)
	if err != nil {
		return fmt.Errorf("insert pii: %w", err)
	}

	return tx.Commit()
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
		`SELECT p.encrypted_name, p.encrypted_ni, p.encrypted_dob, p.encrypted_address, p.encrypted_email, p.encrypted_phone
		 FROM cust_pii p JOIN cust_customers c ON p.customer_id = c.id WHERE c.ref = $1`, ref)
	return s.decryptPIIRow(row)
}

// GetPIIByID retrieves PII by customer primary key ID.
func (s *SQLCustomerStore) GetPIIByID(ctx context.Context, id string) (*PIIData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT encrypted_name, encrypted_ni, encrypted_dob, encrypted_address, encrypted_email, encrypted_phone
		 FROM cust_pii WHERE customer_id = $1`, id)
	return s.decryptPIIRow(row)
}

func (s *SQLCustomerStore) decryptPIIRow(row *sql.Row) (*PIIData, error) {
	var encName, encNI, encDOB, encAddr, encEmail, encPhone string
	err := row.Scan(&encName, &encNI, &encDOB, &encAddr, &encEmail, &encPhone)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan pii: %w", err)
	}

	key, err := s.key.PIIKey()
	if err != nil {
		return nil, fmt.Errorf("get key: %w", err)
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
