package banking

import (
	"errors"
	"strings"
	"time"
)

type SourceType string

const (
	SourceInternal SourceType = "INTERNAL"
	SourceBank     SourceType = "BANK"
)

type Direction string

const (
	DirectionCredit Direction = "CREDIT"
	DirectionDebit  Direction = "DEBIT"
)

const CurrencyIDR = "IDR"

var ErrInvalidTransaction = errors.New("invalid transaction")

// Transaction is a validated-shaped input record; amounts are integer rupiah.
type Transaction struct {
	ID          string
	ImportID    string
	ReferenceID string
	Date        time.Time
	Direction   Direction
	Amount      int64
	Currency    string
	Source      SourceType
}

// Account is synthetic reference data and never contains bank credentials.
type Account struct {
	ID                  string
	Code                string
	BankDisplayName     string
	MaskedAccountNumber string
	AccountHolderName   string
	Currency            string
	TimeZone            string
	IsActive            bool
}

// ValidateTransaction enforces the v1 source, direction, and IDR amount scope.
func ValidateTransaction(transaction Transaction) error {
	if strings.TrimSpace(transaction.ReferenceID) == "" || transaction.Date.IsZero() {
		return ErrInvalidTransaction
	}
	if transaction.Source != SourceInternal && transaction.Source != SourceBank {
		return ErrInvalidTransaction
	}
	if transaction.Direction != DirectionCredit && transaction.Direction != DirectionDebit {
		return ErrInvalidTransaction
	}
	if transaction.Amount <= 0 || transaction.Currency != CurrencyIDR {
		return ErrInvalidTransaction
	}
	return nil
}
