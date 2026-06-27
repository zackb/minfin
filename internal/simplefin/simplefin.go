// Package simplefin is a minimal client for the SimpleFIN protocol:
// https://www.simplefin.org/protocol.html
package simplefin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type AccountSet struct {
	Errors   []string  `json:"errors"`
	Accounts []Account `json:"accounts"`
}

type Account struct {
	Org struct {
		Domain string `json:"domain"`
		Name   string `json:"name"`
	} `json:"org"`
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Currency     string        `json:"currency"`
	Balance      string        `json:"balance"`
	BalanceDate  int64         `json:"balance-date"`
	Transactions []Transaction `json:"transactions"`
}

type Transaction struct {
	ID          string `json:"id"`
	Posted      int64  `json:"posted"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
	Payee       string `json:"payee"`
	Memo        string `json:"memo"`
	Pending     bool   `json:"pending"`
}

// Claim trades a base64 setup token for a long-lived access URL.
func Claim(token string) (string, error) {
	claimURL, err := base64.StdEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return "", fmt.Errorf("decode setup token: %w", err)
	}
	resp, err := http.Post(string(claimURL), "text/plain", nil)
	if err != nil {
		return "", fmt.Errorf("claim: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claim returned %s: %s", resp.Status, body)
	}
	access := strings.TrimSpace(string(body))
	if _, err := url.Parse(access); err != nil {
		return "", fmt.Errorf("invalid access url: %w", err)
	}
	return access, nil
}

// Fetch reads accounts (with transactions) from the access URL.
// startDays>0 limits transactions to the last N days; 0 fetches the default window.
func Fetch(accessURL string, startDays int) (AccountSet, error) {
	var set AccountSet
	u, err := url.Parse(strings.TrimSpace(accessURL) + "/accounts")
	if err != nil {
		return set, fmt.Errorf("parse access url: %w", err)
	}
	if startDays > 0 {
		q := u.Query()
		q.Set("start-date", strconv.FormatInt(time.Now().AddDate(0, 0, -startDays).Unix(), 10))
		u.RawQuery = q.Encode()
	}
	resp, err := http.Get(u.String())
	if err != nil {
		return set, fmt.Errorf("fetch accounts: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return set, fmt.Errorf("accounts returned %s: %s", resp.Status, body)
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return set, fmt.Errorf("decode accounts: %w", err)
	}
	return set, nil
}
