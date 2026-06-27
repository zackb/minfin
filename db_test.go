package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestParseCents(t *testing.T) {
	cases := map[string]int64{
		"-12.34": -1234, "100": 10000, "5.6": 560,
		"0.05": 5, "": 0, "+3.20": 320, "-0.99": -99,
	}
	for in, want := range cases {
		got, err := parseCents(in)
		if err != nil || got != want {
			t.Errorf("parseCents(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
}

func seedDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	at := func(s string) int64 {
		ts, _ := time.Parse(time.RFC3339, s)
		return ts.Unix()
	}
	set := AccountSet{Accounts: []Account{{
		ID: "a1", Name: "Checking", Balance: "0",
		Transactions: []Transaction{
			{ID: "t1", Posted: at("2026-06-10T12:00:00Z"), Amount: "-10.00", Payee: "Coffee"},
			{ID: "t2", Posted: at("2026-06-10T15:00:00Z"), Amount: "-5.00", Payee: "Coffee"},
			{ID: "t3", Posted: at("2026-06-11T09:00:00Z"), Amount: "-20.00", Payee: "Grocer"},
			{ID: "t4", Posted: at("2026-06-10T10:00:00Z"), Amount: "100.00", Payee: "Paycheck"},
		},
	}}}
	if err := db.SaveAccountSet(set); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSpendingSeries(t *testing.T) {
	db := seedDB(t)
	start := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	s, err := db.SpendingSeries(start, end, "daily", false)
	if err != nil {
		t.Fatal(err)
	}
	wantLabels := []string{"2026-06-10", "2026-06-11"}
	if len(s.Labels) != 2 || s.Labels[0] != wantLabels[0] || s.Labels[1] != wantLabels[1] {
		t.Fatalf("labels = %v, want %v", s.Labels, wantLabels)
	}
	if len(s.Lines) != 1 || s.Lines[0].Values[0] != 15 || s.Lines[0].Values[1] != 20 {
		t.Fatalf("values = %+v, want [15 20] (credit excluded)", s.Lines)
	}
}

func TestTopPayees(t *testing.T) {
	db := seedDB(t)
	start := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	p, err := db.TopPayees(start, end, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 2 || p[0].Payee != "Coffee" || p[0].Count != 2 || p[0].Spent != 15 {
		t.Fatalf("top payee = %+v, want Coffee x2 $15", p)
	}
}

func TestResolveRange(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // Wednesday
	start, end := resolveRange("this-week", now)
	if start.Format("2006-01-02") != "2026-06-15" { // Monday
		t.Errorf("this-week start = %s, want 2026-06-15", start.Format("2006-01-02"))
	}
	if end.Format("2006-01-02") != "2026-06-18" {
		t.Errorf("this-week end = %s, want 2026-06-18", end.Format("2006-01-02"))
	}
	start, _ = resolveRange("last-30-days", now)
	if start.Format("2006-01-02") != "2026-05-19" {
		t.Errorf("last-30-days start = %s, want 2026-05-19", start.Format("2006-01-02"))
	}
}
