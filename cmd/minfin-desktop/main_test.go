package main

import (
	"path/filepath"
	"testing"

	"github.com/zackb/minfin/internal/store"
)

// A portfolio created without membership (as the GTK app does) must become
// visible to the desktop's local user after adoption.
func TestAdoptPortfolios(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := st.CreatePortfolio("", "") // no AddMember, like GTK
	if err != nil {
		t.Fatal(err)
	}

	uid, err := ensureLocalUser(st)
	if err != nil {
		t.Fatal(err)
	}
	if ps, _ := st.PortfoliosForUser(uid); len(ps) != 0 {
		t.Fatalf("want 0 portfolios before adoption, got %d", len(ps))
	}

	if err := adoptPortfolios(st, uid); err != nil {
		t.Fatal(err)
	}
	ps, err := st.PortfoliosForUser(uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].ID != pid {
		t.Fatalf("want portfolio %s adopted, got %+v", pid, ps)
	}

	// Idempotent: running again doesn't duplicate membership.
	if err := adoptPortfolios(st, uid); err != nil {
		t.Fatal(err)
	}
	if ps, _ := st.PortfoliosForUser(uid); len(ps) != 1 {
		t.Fatalf("want 1 portfolio after re-adopt, got %d", len(ps))
	}
}
