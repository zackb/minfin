// Command minfin-seed writes a portfolio of sample accounts to a SQLite file so
// the local clients have something to show. Dev/demo only — it goes through the
// same store.SaveAccountSet the syncer uses, so the file matches a real one.
//
// Usage: minfin-seed <db-path>   (overwrite the file first for a clean slate)
package main

import (
	"log"
	"os"
	"time"

	"github.com/zackb/minfin/internal/simplefin"
	"github.com/zackb/minfin/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: minfin-seed <db-path>")
	}
	st, err := store.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	pid, err := st.CreatePortfolio("Demo", "https://example.invalid")
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now().Unix()
	chk := simplefin.Account{ID: "chk", Name: "Checking", Currency: "USD", Balance: "4210.55", BalanceDate: now,
		Transactions: []simplefin.Transaction{
			{ID: "t1", Posted: now, Amount: "-42.10", Payee: "Coffee Bar"},
			{ID: "t2", Posted: now, Amount: "-128.74", Payee: "Grocery"},
			{ID: "t3", Posted: now, Amount: "2400.00", Payee: "Paycheck"},
		}}
	sav := simplefin.Account{ID: "sav", Name: "Savings", Currency: "USD", Balance: "18000.00", BalanceDate: now}
	card := simplefin.Account{ID: "card", Name: "Credit Card", Currency: "USD", Balance: "-1325.40", BalanceDate: now}
	chk.Org.Name, sav.Org.Name, card.Org.Name = "Demo Bank", "Demo Bank", "Demo Bank"

	if err := st.SaveAccountSet(pid, simplefin.AccountSet{Accounts: []simplefin.Account{chk, sav, card}}); err != nil {
		log.Fatal(err)
	}
	log.Printf("seeded %s", os.Args[1])
}
