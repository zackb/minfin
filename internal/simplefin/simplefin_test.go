package simplefin

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

// One self-check covering the two money-path bits: token decode + JSON parse.
// Uses a TLS server on loopback: the SSRF-hardened client blocks loopback and
// requires https, so we swap in the test server's client (trusts its cert, no
// Control hook) for the duration.
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

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/claim":
			w.Write([]byte("https://" + r.Host)) // access URL = this server's base (https)
		case "/accounts":
			w.Write([]byte(sampleJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	orig := client
	client = srv.Client()
	defer func() { client = orig }()

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

func TestRequireHTTPS(t *testing.T) {
	ok := []string{"https://bridge.simplefin.org/x", "https://user:pw@host.example/access"}
	bad := []string{"http://example.com", "ftp://example.com", "://nope", "https://"}
	for _, u := range ok {
		if err := requireHTTPS(u); err != nil {
			t.Errorf("requireHTTPS(%q) = %v, want nil", u, err)
		}
	}
	for _, u := range bad {
		if err := requireHTTPS(u); err == nil {
			t.Errorf("requireHTTPS(%q) = nil, want error", u)
		}
	}
}

func TestClaimRejectsNonPublicAndNonHTTPS(t *testing.T) {
	// http scheme rejected before any dial.
	tok := base64.StdEncoding.EncodeToString([]byte("http://169.254.169.254/latest/meta-data/"))
	if _, err := Claim(tok); err == nil {
		t.Fatal("Claim accepted an http metadata URL, want error")
	}
	// https but link-local metadata IP: rejected by the dialer's Control hook.
	tok = base64.StdEncoding.EncodeToString([]byte("https://169.254.169.254/latest/meta-data/"))
	if _, err := Claim(tok); err == nil {
		t.Fatal("Claim reached a link-local metadata IP, want error")
	}
}
