package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zackb/minfin/internal/store"
)

// TestSeed runs the seeder logic against a throwaway DB and checks the money +
// category path: login user exists, the portfolio is linked, all 7 accounts
// land, and the spend pie has a believable spread of categories.
func TestSeed(t *testing.T) {
	db := filepath.Join(t.TempDir(), "seed.db")
	old := os.Args
	os.Args = []string{"minfin-seed", db}
	main()
	os.Args = old

	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	u, err := st.UserByEmail("demo@example.com")
	if err != nil {
		t.Fatalf("demo user not found: %v", err)
	}
	ports, err := st.PortfoliosForUser(u.ID)
	if err != nil || len(ports) != 1 {
		t.Fatalf("PortfoliosForUser = %d, %v; want 1", len(ports), err)
	}
	pid := ports[0].ID

	accts, err := st.Accounts(pid, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(accts) != 7 {
		t.Fatalf("accounts = %d; want 7", len(accts))
	}

	stats, err := st.SpendByCategory(pid, time.Now().AddDate(0, 0, -90), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) < 4 {
		t.Fatalf("spend categories = %d; want >3", len(stats))
	}
}
