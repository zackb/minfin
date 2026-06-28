package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrEmailTaken is returned when registering an email that already exists.
var ErrEmailTaken = errors.New("email already registered")

// ErrNoUser is returned when a user lookup finds nothing.
var ErrNoUser = errors.New("user not found")

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// CreateUser inserts a user with a pre-hashed password. Email is unique
// case-insensitively; a duplicate returns ErrEmailTaken.
func (s *Store) CreateUser(email, passwordHash string) (User, error) {
	u := User{ID: uuid.NewString(), Email: email, PasswordHash: passwordHash, CreatedAt: time.Now()}
	_, err := s.db.Exec(
		`INSERT INTO users(id, email, password_hash, created_at) VALUES(?,?,?,?)`,
		u.ID, u.Email, u.PasswordHash, u.CreatedAt.Unix())
	if err != nil {
		// modernc sqlite reports a UNIQUE constraint failure on the email index.
		return User{}, ErrEmailTaken
	}
	return u, nil
}

func (s *Store) UserByEmail(email string) (User, error) {
	return s.scanUser(`SELECT id, email, password_hash, created_at FROM users WHERE lower(email)=lower(?)`, email)
}

func (s *Store) UserByID(id string) (User, error) {
	return s.scanUser(`SELECT id, email, password_hash, created_at FROM users WHERE id=?`, id)
}

func (s *Store) scanUser(q, arg string) (User, error) {
	var u User
	var created int64
	err := s.db.QueryRow(q, arg).Scan(&u.ID, &u.Email, &u.PasswordHash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNoUser
	}
	if err != nil {
		return User{}, err
	}
	u.CreatedAt = time.Unix(created, 0)
	return u, nil
}
