package customers

import (
	"context"
	"time"
)

// CustomerRecord holds non-sensitive customer data.
type CustomerRecord struct {
	ID            string
	Ref           string // human-readable: "cust-001"
	JoinDate      time.Time
	KYCVerified   bool
	KYCLastCheck  time.Time
	KYCRiskRating string
}

// PIIInput is the input struct for storing PII fields.
type PIIInput struct {
	Name    string
	NI      string
	DOB     string
	Address string
	Email   string
	Phone   string
}

// PIIData is the output struct for retrieving decrypted PII fields.
type PIIData struct {
	Name    string
	NI      string
	DOB     string
	Address string
	Email   string
	Phone   string
}

// CustomerStore defines the interface for customer persistence.
type CustomerStore interface {
	Create(ctx context.Context, cust CustomerRecord, pii PIIInput) error
	Get(ctx context.Context, ref string) (*CustomerRecord, error)
	GetPII(ctx context.Context, ref string) (*PIIData, error)
	GetName(ctx context.Context, ref string) (string, error)
	List(ctx context.Context, offset, limit int) ([]CustomerRecord, int, error)
	Count(ctx context.Context) (int, error)
	Reset(ctx context.Context) error
	RotateKeys(ctx context.Context) (int, error)
}
