// Package resource contains the database and callback contracts shared by
// generated resources and business modules.
package resource

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	errNilStoreContext  = errors.New("resource: context is nil")
	errNilStoreDB       = errors.New("resource: store database is nil")
	errNilTransactionFn = errors.New("resource: transaction callback is nil")
)

// Session is the smallest database session accepted by resource repository
// methods. A Store is used for ordinary reads; a Tx is used inside a
// transaction callback.
type Session interface {
	DB() *gorm.DB
}

// Store borrows an already-open GORM connection. The caller owns and closes
// the connection.
type Store struct {
	db *gorm.DB
}

// Tx is the transaction handle created by InTransaction. It is valid only
// while the transaction callback is running.
type Tx struct {
	db *gorm.DB
}

// NewStore creates a Store that borrows db. It does not open or close a
// connection.
func NewStore(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, errNilStoreDB
	}
	return &Store{db: db}, nil
}

// DB returns the borrowed GORM handle. A zero Store returns nil.
func (s *Store) DB() *gorm.DB {
	if s == nil {
		return nil
	}
	return s.db
}

// InTransaction runs fn on a GORM transaction using ctx. A callback error is
// returned unchanged so callers can use errors.Is; GORM rolls the transaction
// back. The connection lifecycle remains owned by the caller.
func (s *Store) InTransaction(ctx context.Context, fn func(Tx) error) error {
	if s == nil || s.db == nil {
		return errNilStoreDB
	}
	if ctx == nil {
		return errNilStoreContext
	}
	if fn == nil {
		return errNilTransactionFn
	}
	return s.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		return fn(Tx{db: db})
	})
}

// DB returns the transaction's GORM handle. The zero Tx returns nil.
func (tx Tx) DB() *gorm.DB {
	return tx.db
}

var (
	_ Session = (*Store)(nil)
	_ Session = Tx{}
)
