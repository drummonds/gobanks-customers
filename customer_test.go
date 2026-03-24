package customers

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/drummonds/go-postgres"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pglike", "file::memory:?_pragma=temp_store(2)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testKey() KeyProvider {
	return FixedKeyProvider{Key: []byte("test-key-32bytes-for-aes256!!!!!")}
}

func TestCreateAndGet(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLCustomerStore(db, testKey())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	cust := CustomerRecord{
		ID:            "id-001",
		Ref:           "cust-001",
		JoinDate:      now,
		KYCVerified:   true,
		KYCLastCheck:  now,
		KYCRiskRating: "Low",
	}
	pii := PIIInput{
		Name:    "Alice Smith",
		NI:      "AB123456C",
		DOB:     "1990-05-15",
		Address: "10 High Street, London, SW1 1AA",
		Email:   "alice@example.com",
		Phone:   "07123 456789",
	}

	if err := store.Create(ctx, cust, pii); err != nil {
		t.Fatal(err)
	}

	// Get by ref
	got, err := store.Get(ctx, "cust-001")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected customer, got nil")
	}
	if got.ID != "id-001" {
		t.Errorf("ID = %q, want %q", got.ID, "id-001")
	}
	if got.Ref != "cust-001" {
		t.Errorf("Ref = %q, want %q", got.Ref, "cust-001")
	}
	if !got.KYCVerified {
		t.Error("expected KYCVerified = true")
	}

	// Get by ID
	got2, err := store.GetByID(ctx, "id-001")
	if err != nil {
		t.Fatal(err)
	}
	if got2 == nil || got2.Ref != "cust-001" {
		t.Errorf("GetByID returned unexpected result: %+v", got2)
	}
}

func TestGetPII(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLCustomerStore(db, testKey())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	cust := CustomerRecord{ID: "id-002", Ref: "cust-002", JoinDate: now, KYCVerified: true, KYCLastCheck: now, KYCRiskRating: "Low"}
	pii := PIIInput{Name: "Bob Jones", NI: "CD654321B", DOB: "1985-03-20", Address: "5 Station Road, Manchester, M1 2AB", Email: "bob@example.com", Phone: "07987 654321"}

	if err := store.Create(ctx, cust, pii); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetPII(ctx, "cust-002")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected PII, got nil")
	}
	if got.Name != "Bob Jones" {
		t.Errorf("Name = %q, want %q", got.Name, "Bob Jones")
	}
	if got.NI != "CD654321B" {
		t.Errorf("NI = %q, want %q", got.NI, "CD654321B")
	}
	if got.Email != "bob@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "bob@example.com")
	}

	// GetPIIByID
	got2, err := store.GetPIIByID(ctx, "id-002")
	if err != nil {
		t.Fatal(err)
	}
	if got2 == nil || got2.Name != "Bob Jones" {
		t.Errorf("GetPIIByID returned unexpected result: %+v", got2)
	}
}

func TestGetName(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLCustomerStore(db, testKey())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	cust := CustomerRecord{ID: "id-003", Ref: "cust-003", JoinDate: now, KYCVerified: true, KYCLastCheck: now, KYCRiskRating: "Standard"}
	pii := PIIInput{Name: "Carol White", NI: "EF111111A", DOB: "1970-12-01", Address: "1 Park Avenue", Email: "carol@example.com", Phone: "07111 222333"}

	if err := store.Create(ctx, cust, pii); err != nil {
		t.Fatal(err)
	}

	name, err := store.GetName(ctx, "cust-003")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Carol White" {
		t.Errorf("GetName = %q, want %q", name, "Carol White")
	}

	// GetNameByID
	name2, err := store.GetNameByID(ctx, "id-003")
	if err != nil {
		t.Fatal(err)
	}
	if name2 != "Carol White" {
		t.Errorf("GetNameByID = %q, want %q", name2, "Carol White")
	}

	// Non-existent returns ref as fallback
	name3, err := store.GetName(ctx, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if name3 != "nonexistent" {
		t.Errorf("GetName fallback = %q, want %q", name3, "nonexistent")
	}
}

func TestListAndCount(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLCustomerStore(db, testKey())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	for i := range 5 {
		cust := CustomerRecord{
			ID: fmt.Sprintf("id-%03d", i), Ref: fmt.Sprintf("cust-%03d", i),
			JoinDate: now, KYCVerified: true, KYCLastCheck: now, KYCRiskRating: "Low",
		}
		pii := PIIInput{
			Name: fmt.Sprintf("Customer %d", i), NI: fmt.Sprintf("XX%06dC", i),
			DOB: "2000-01-01", Address: "Test St", Email: "test@test.com", Phone: "07000000000",
		}
		if err := store.Create(ctx, cust, pii); err != nil {
			t.Fatal(err)
		}
	}

	count, err := store.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Errorf("Count = %d, want 5", count)
	}

	list, total, err := store.List(ctx, 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Errorf("List total = %d, want 5", total)
	}
	if len(list) != 3 {
		t.Errorf("List len = %d, want 3", len(list))
	}

	// Page 2
	list2, total2, err := store.List(ctx, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	if total2 != 5 {
		t.Errorf("List total2 = %d, want 5", total2)
	}
	if len(list2) != 2 {
		t.Errorf("List2 len = %d, want 2", len(list2))
	}
}

func TestReset(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLCustomerStore(db, testKey())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	cust := CustomerRecord{ID: "id-r1", Ref: "cust-r1", JoinDate: now, KYCVerified: true, KYCLastCheck: now, KYCRiskRating: "Low"}
	pii := PIIInput{Name: "Reset Test", NI: "RR000000A", DOB: "2000-01-01", Address: "Test", Email: "r@test.com", Phone: "07000000000"}

	if err := store.Create(ctx, cust, pii); err != nil {
		t.Fatal(err)
	}

	count, _ := store.Count(ctx)
	if count != 1 {
		t.Fatalf("Count before reset = %d, want 1", count)
	}

	if err := store.Reset(ctx); err != nil {
		t.Fatal(err)
	}

	count, _ = store.Count(ctx)
	if count != 0 {
		t.Errorf("Count after reset = %d, want 0", count)
	}
}

func TestNIHashDedup(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLCustomerStore(db, testKey())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	ni := "AB123456C"

	cust1 := CustomerRecord{ID: "id-d1", Ref: "cust-d1", JoinDate: now, KYCVerified: true, KYCLastCheck: now, KYCRiskRating: "Low"}
	pii1 := PIIInput{Name: "Dup 1", NI: ni, DOB: "2000-01-01", Address: "Test", Email: "d1@test.com", Phone: "07000000001"}
	if err := store.Create(ctx, cust1, pii1); err != nil {
		t.Fatal(err)
	}

	// Verify NI hash is stored and queryable
	niH := hashNI(ni)
	var count int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cust_pii WHERE ni_hash = $1`, niH).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("NI hash count = %d, want 1", count)
	}
}
