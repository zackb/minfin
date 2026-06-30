// Command minfin-seed writes a demo portfolio to a SQLite file so the local
// clients have something realistic to show: a static login, asset-backed loans
// (house, car), retirement + brokerage assets, and ~90 days of categorized
// transactions that fill the spend/income pie charts. Dev/demo only — accounts
// and transactions go through the same store.SaveAccountSet the syncer uses, so
// the file matches a real one.
//
// Login: demo@example.com / password
//
// Usage: minfin-seed <db-path>   (overwrite the file first for a clean slate)
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zackb/minfin/internal/auth"
	"github.com/zackb/minfin/internal/simplefin"
	"github.com/zackb/minfin/internal/store"
)

// account describes a seed account plus the type/asset-value columns that
// aren't part of simplefin.Account and have to be set after SaveAccountSet.
type account struct {
	id, name, org, typ string
	balance            float64 // dollars; computed for the card from its txns
	asset              float64 // underlying asset value (house/car), 0 if none
}

// txn is one seed transaction carrying the category to assign after save.
// amount is signed dollars: negative = spend, positive = income. daysAgo dates
// it relative to now.
type txn struct {
	acct, payee, cat string
	amount           float64
	daysAgo          int
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: minfin-seed <db-path>")
	}
	st, err := store.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	// Static demo user, idempotent so re-seeding a file doesn't fail.
	hash, err := auth.HashPassword("password")
	if err != nil {
		log.Fatal(err)
	}
	u, err := st.CreateUser("demo@example.com", hash)
	if errors.Is(err, store.ErrEmailTaken) {
		u, err = st.UserByEmail("demo@example.com")
	}
	if err != nil {
		log.Fatal(err)
	}

	pid, err := st.CreatePortfolio("Demo", "https://example.invalid")
	if err != nil {
		log.Fatal(err)
	}
	if err := st.AddMember(pid, u.ID, "owner"); err != nil {
		log.Fatal(err)
	}

	accts := []account{
		{"chk", "Checking", "Demo Bank", "checking", 3800, 0},
		{"sav", "Savings", "Demo Bank", "savings", 18000, 0},
		{"card", "Credit Card", "Demo Bank", "credit_card", 0, 0}, // balance from txns
		{"mtg", "Home Mortgage", "Demo Mortgage", "mortgage", -312500, 465000},
		{"auto", "Auto Loan", "Demo Auto Finance", "auto_loan", -18400, 27000},
		{"k401", "401(k)", "Demo Brokerage", "investment", 142300, 0},
		{"brk", "Brokerage", "Demo Brokerage", "investment", 36750, 0},
	}

	txns := seedTxns()

	// The card balance should match the transactions shown on it.
	var cardBal float64
	for _, t := range txns {
		if t.acct == "card" {
			cardBal += t.amount
		}
	}

	now := time.Now()
	set := simplefin.AccountSet{Accounts: make([]simplefin.Account, len(accts))}
	byID := map[string]*simplefin.Account{}
	for i, a := range accts {
		bal := a.balance
		if a.id == "card" {
			bal = cardBal
		}
		set.Accounts[i] = simplefin.Account{ID: a.id, Name: a.name, Currency: "USD",
			Balance: fmt.Sprintf("%.2f", bal), BalanceDate: now.Unix()}
		set.Accounts[i].Org.Name = a.org
		byID[a.id] = &set.Accounts[i]
	}

	type cat struct{ id, name string }
	var cats []cat
	for i, t := range txns {
		id := fmt.Sprintf("t%03d", i)
		byID[t.acct].Transactions = append(byID[t.acct].Transactions, simplefin.Transaction{
			ID:     id,
			Posted: now.AddDate(0, 0, -t.daysAgo).Unix(),
			Amount: fmt.Sprintf("%.2f", t.amount),
			Payee:  t.payee,
		})
		cats = append(cats, cat{id, t.cat})
	}

	if err := st.SaveAccountSet(pid, set); err != nil {
		log.Fatal(err)
	}
	for i := range accts {
		a := accts[i]
		if err := st.SetAccountType(pid, a.id, a.typ); err != nil {
			log.Fatal(err)
		}
		if a.asset != 0 {
			if err := st.SetAccountAssetValue(pid, a.id, int64(a.asset*100)); err != nil {
				log.Fatal(err)
			}
		}
	}
	for _, c := range cats {
		if err := st.SetTxnCategory(pid, c.id, c.name); err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("seeded %s (login demo@example.com / password)", os.Args[1])
}

// seedTxns builds ~90 days of realistic, recurring activity weighted so the
// spend pie has a believable shape (Groceries/Restaurants/Bills largest).
func seedTxns() []txn {
	var ts []txn

	// Bi-weekly paycheck → checking.
	for d := 5; d <= 90; d += 14 {
		ts = append(ts, txn{"chk", "Acme Corp Payroll", "Paycheck", 2400, d})
	}
	// Monthly recurring (each of the last 3 months).
	for m, d := 0, 8; m < 3; m, d = m+1, d+30 {
		ts = append(ts,
			txn{"chk", "City Power & Light", "Bills & Utilities", -118.40, d},
			txn{"chk", "Fiber Internet", "Bills & Utilities", -69.99, d + 2},
			txn{"chk", "Mobile Wireless", "Bills & Utilities", -85.00, d + 3},
			txn{"chk", "Demo Mortgage Pmt", "Rent & Mortgage", -1820.00, d + 1},
			txn{"chk", "Demo Auto Finance", "Auto & Transport", -412.00, d + 4},
			txn{"chk", "SafeWay Insurance", "Auto & Transport", -134.00, d + 6},
			txn{"chk", "Transfer to Savings", "Transfer", -500.00, d + 5},
		)
	}
	// Weekly groceries + gas → card.
	for w, d := 0, 4; d <= 88; w, d = w+1, d+7 {
		groc := []struct {
			p string
			a float64
		}{{"Whole Foods Market", -132.55}, {"Trader Joe's", -96.20}, {"Costco Wholesale", -211.80}}[w%3]
		ts = append(ts, txn{"card", groc.p, "Groceries", groc.a, d})
		gas := []struct {
			p string
			a float64
		}{{"Shell", -52.30}, {"Chevron", -48.75}, {"Costco Gas", -44.10}}[w%3]
		ts = append(ts, txn{"card", gas.p, "Gas & Fuel", gas.a, d + 1})
	}
	// ~2x/week restaurants → card.
	rest := []struct {
		p string
		a float64
	}{
		{"Blue Bottle Coffee", -6.85}, {"Chipotle", -14.40}, {"Sushi Ya", -58.20},
		{"Pizza Place", -27.60}, {"Thai Basil", -41.10}, {"Local Diner", -22.35},
		{"Taco Truck", -12.00}, {"Ramen House", -33.80},
	}
	for i, d := 0, 3; d <= 88; i, d = i+1, d+4 {
		r := rest[i%len(rest)]
		ts = append(ts, txn{"card", r.p, "Restaurants", r.a, d})
	}
	// Occasional shopping / health / entertainment / travel → card.
	ts = append(ts,
		txn{"card", "Amazon", "Shopping", -78.40, 12},
		txn{"card", "Target", "Shopping", -53.90, 41},
		txn{"card", "Amazon", "Shopping", -34.20, 67},
		txn{"card", "Walgreens Pharmacy", "Health", -24.80, 19},
		txn{"card", "Dr. Smith Dental", "Health", -150.00, 52},
		txn{"card", "Netflix", "Entertainment", -15.99, 10},
		txn{"card", "Netflix", "Entertainment", -15.99, 40},
		txn{"card", "Spotify", "Entertainment", -10.99, 22},
		txn{"card", "AMC Theatres", "Entertainment", -38.50, 33},
		txn{"card", "United Airlines", "Travel", -342.00, 28},
		txn{"card", "Marriott Hotels", "Travel", -218.60, 27},
	)
	return ts
}
