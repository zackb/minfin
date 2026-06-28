package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNoPortfolio is returned when a portfolio lookup finds nothing.
var ErrNoPortfolio = errors.New("portfolio not found")

// Portfolio is a group of accounts tied to one SimpleFIN token. access_url and
// the sync status that used to live in the global meta table are columns here.
type Portfolio struct {
	ID        string
	Name      string
	AccessURL string
	Sync      SyncStatus
}

// CreatePortfolio inserts a portfolio and returns its id. Categories are seeded
// for it so a fresh portfolio has the default labels.
func (s *Store) CreatePortfolio(name, accessURL string) (string, error) {
	id := uuid.NewString()
	if _, err := s.db.Exec(
		`INSERT INTO portfolios(id, name, access_url, created_at) VALUES(?,?,?,?)`,
		id, name, accessURL, time.Now().Unix()); err != nil {
		return "", err
	}
	if err := s.seedCategories(id); err != nil {
		return "", err
	}
	return id, nil
}

// AddMember grants a user access to a portfolio with the given role.
func (s *Store) AddMember(portfolioID, userID, role string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO portfolio_members(portfolio_id, user_id, role) VALUES(?,?,?)`,
		portfolioID, userID, role)
	return err
}

// PortfoliosForUser lists the portfolios a user can access, newest first.
func (s *Store) PortfoliosForUser(userID string) ([]Portfolio, error) {
	return s.scanPortfolios(
		`SELECT p.id, p.name, p.access_url, p.sync_at, p.sync_errors
		 FROM portfolios p JOIN portfolio_members m ON m.portfolio_id = p.id
		 WHERE m.user_id = ? ORDER BY p.created_at DESC`, userID)
}

// Portfolios lists every portfolio (used by the sync loop).
func (s *Store) Portfolios() ([]Portfolio, error) {
	return s.scanPortfolios(
		`SELECT id, name, access_url, sync_at, sync_errors FROM portfolios ORDER BY created_at`)
}

func (s *Store) PortfolioByID(id string) (Portfolio, error) {
	ps, err := s.scanPortfolios(
		`SELECT id, name, access_url, sync_at, sync_errors FROM portfolios WHERE id = ?`, id)
	if err != nil {
		return Portfolio{}, err
	}
	if len(ps) == 0 {
		return Portfolio{}, ErrNoPortfolio
	}
	return ps[0], nil
}

func (s *Store) scanPortfolios(q string, args ...any) ([]Portfolio, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Portfolio
	for rows.Next() {
		var p Portfolio
		var syncAt int64
		var syncErrs string
		if err := rows.Scan(&p.ID, &p.Name, &p.AccessURL, &syncAt, &syncErrs); err != nil {
			return nil, err
		}
		if syncAt > 0 {
			p.Sync.At = time.Unix(syncAt, 0)
		}
		if syncErrs != "" {
			_ = json.Unmarshal([]byte(syncErrs), &p.Sync.Errors)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) SetPortfolioAccessURL(id, url string) error {
	_, err := s.db.Exec(`UPDATE portfolios SET access_url=? WHERE id=?`, url, id)
	return err
}

// SetPortfolioSyncStatus records the outcome of a sync on the portfolio row.
func (s *Store) SetPortfolioSyncStatus(id string, st SyncStatus) error {
	errs, _ := json.Marshal(st.Errors)
	_, err := s.db.Exec(`UPDATE portfolios SET sync_at=?, sync_errors=? WHERE id=?`,
		st.At.Unix(), string(errs), id)
	return err
}

// PortfolioSyncStatus returns the last sync status for a portfolio.
func (s *Store) PortfolioSyncStatus(id string) (SyncStatus, error) {
	var syncAt int64
	var syncErrs string
	err := s.db.QueryRow(`SELECT sync_at, sync_errors FROM portfolios WHERE id=?`, id).Scan(&syncAt, &syncErrs)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncStatus{}, nil
	}
	if err != nil {
		return SyncStatus{}, err
	}
	var st SyncStatus
	if syncAt > 0 {
		st.At = time.Unix(syncAt, 0)
	}
	if syncErrs != "" {
		_ = json.Unmarshal([]byte(syncErrs), &st.Errors)
	}
	return st, nil
}
