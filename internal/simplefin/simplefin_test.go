package simplefin

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

// One self-check covering the two money-path bits: token decode + JSON parse.
func TestClaimAndFetch(t *testing.T) {
	const sampleJSON = `{
	  "errors": [],
	  "accounts": [{
	    "org": {"domain": "mybank.com", "name": "My Bank"},
	    "id": "acct-1", "name": "Checking", "currency": "USD",
	    "balance": "100.23", "balance-date": 1700000000,
	    "transactions": [
	      {"id": "t1", "posted": 1699900000, "amount": "-12.34", "description": "Coffee", "payee": "Cafe", "pending": false}
	    ]
	  }]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/claim":
			w.Write([]byte("http://" + r.Host)) // access URL = this server's base
		case "/accounts":
			w.Write([]byte(sampleJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	token := base64.StdEncoding.EncodeToString([]byte(srv.URL + "/claim"))
	if _, err := Claim(token); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	set, err := Fetch(srv.URL, 0)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(set.Accounts) != 1 {
		t.Fatalf("want 1 account, got %d", len(set.Accounts))
	}
	a := set.Accounts[0]
	if a.Balance != "100.23" || a.Org.Name != "My Bank" {
		t.Fatalf("bad account parse: %+v", a)
	}
	if len(a.Transactions) != 1 || a.Transactions[0].Amount != "-12.34" {
		t.Fatalf("bad txn parse: %+v", a.Transactions)
	}
}
