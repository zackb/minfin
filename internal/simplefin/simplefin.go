// Package simplefin is a minimal client for the SimpleFIN protocol:
// https://www.simplefin.org/protocol.html
package simplefin

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// errBlockedHost rejects a request whose target resolves to a non-public IP.
var errBlockedHost = errors.New("refusing to connect to a non-public address")

// client is an SSRF-hardened HTTP client for the SimpleFIN calls. The setup
// token is user-supplied, so without this a caller could aim the server at
// internal hosts (cloud metadata at 169.254.169.254, localhost, RFC1918). The
// Control hook runs after DNS resolution on the actual dialed IP, so it also
// stops DNS-rebinding TOCTOU (a name that resolves public once, private next).
var client = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return err
				}
				ip := net.ParseIP(host)
				if ip == nil || !publicIP(ip) {
					return fmt.Errorf("%w: %s", errBlockedHost, host)
				}
				return nil
			},
		}).DialContext,
	},
}

// publicIP reports whether ip is a routable public address (not loopback,
// private, link-local, multicast, or unspecified).
func publicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified())
}

// requireHTTPS parses raw and rejects anything that isn't a plain https URL, so
// a claim/access URL can never downgrade bank traffic to cleartext.
func requireHTTPS(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("url must be https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("url missing host")
	}
	return nil
}

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

// Claim trades a base64 setup token for a long-lived access URL. The decoded
// claim URL and the returned access URL are both validated as public https
// endpoints (see requireHTTPS / the SSRF-hardened client).
func Claim(token string) (string, error) {
	claimURL, err := base64.StdEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return "", fmt.Errorf("decode setup token: %w", err)
	}
	if err := requireHTTPS(string(claimURL)); err != nil {
		return "", fmt.Errorf("setup token: %w", err)
	}
	resp, err := client.Post(strings.TrimSpace(string(claimURL)), "text/plain", nil)
	if err != nil {
		return "", fmt.Errorf("claim: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		// Deliberately don't echo the upstream body: on a hosted server it could
		// be an internal service's response reflected back to the caller.
		return "", fmt.Errorf("claim returned %s", resp.Status)
	}
	access := strings.TrimSpace(string(body))
	if err := requireHTTPS(access); err != nil {
		return "", fmt.Errorf("invalid access url: %w", err)
	}
	return access, nil
}

// Fetch reads accounts (with transactions) from the access URL.
// startDays>0 limits transactions to the last N days; 0 fetches the default window.
func Fetch(accessURL string, startDays int) (AccountSet, error) {
	var set AccountSet
	if err := requireHTTPS(accessURL); err != nil {
		return set, fmt.Errorf("access url: %w", err)
	}
	u, err := url.Parse(strings.TrimSpace(accessURL) + "/accounts")
	if err != nil {
		return set, fmt.Errorf("parse access url: %w", err)
	}
	if startDays > 0 {
		q := u.Query()
		q.Set("start-date", strconv.FormatInt(time.Now().AddDate(0, 0, -startDays).Unix(), 10))
		u.RawQuery = q.Encode()
	}
	resp, err := client.Get(u.String())
	if err != nil {
		return set, fmt.Errorf("fetch accounts: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode != http.StatusOK {
		return set, fmt.Errorf("accounts returned %s: %s", resp.Status, body)
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return set, fmt.Errorf("decode accounts: %w", err)
	}
	return set, nil
}
