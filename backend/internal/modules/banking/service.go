package banking

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"reconx/backend/internal/platform/audit"
)

var ErrAccountInUse = errors.New("account has active reconciliation batches")
var ErrInvalidAccount = errors.New("invalid bank account")
var ErrAccountExists = errors.New("account code already exists")

type AccountView struct {
	ID                  string `json:"id"`
	Code                string `json:"account_code"`
	BankDisplayName     string `json:"bank_display_name"`
	MaskedAccountNumber string `json:"masked_account_no"`
	AccountHolderName   string `json:"account_holder_name"`
	Currency            string `json:"currency"`
	TimeZone            string `json:"timezone"`
	Active              bool   `json:"active"`
}

type Service struct{ DB *sql.DB }

func (s Service) ListAccounts(ctx context.Context, query string, active *bool) ([]AccountView, error) {
	items, _, err := s.ListAccountsPage(ctx, query, active, 1, 100)
	return items, err
}

func (s Service) ListAccountsPage(ctx context.Context, query string, active *bool, page, pageSize int) ([]AccountView, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM bank_accounts WHERE ($1='' OR account_code ILIKE '%'||$1||'%' OR bank_display_name ILIKE '%'||$1||'%') AND ($2::boolean IS NULL OR active=$2)`, strings.TrimSpace(query), active).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id::text, account_code, bank_display_name, masked_account_no, account_holder_name, currency, timezone, active
		FROM bank_accounts WHERE ($1 = '' OR account_code ILIKE '%'||$1||'%' OR bank_display_name ILIKE '%'||$1||'%')
		AND ($2::boolean IS NULL OR active = $2) ORDER BY account_code LIMIT $3 OFFSET $4`, strings.TrimSpace(query), active, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	accounts := make([]AccountView, 0)
	for rows.Next() {
		var account AccountView
		if err := rows.Scan(&account.ID, &account.Code, &account.BankDisplayName, &account.MaskedAccountNumber, &account.AccountHolderName, &account.Currency, &account.TimeZone, &account.Active); err != nil {
			return nil, 0, err
		}
		accounts = append(accounts, account)
	}
	return accounts, total, rows.Err()
}

func (s Service) GetAccount(ctx context.Context, id string) (AccountView, error) {
	var account AccountView
	err := s.DB.QueryRowContext(ctx, `SELECT id::text, account_code, bank_display_name, masked_account_no, account_holder_name, currency, timezone, active FROM bank_accounts WHERE id=$1`, id).
		Scan(&account.ID, &account.Code, &account.BankDisplayName, &account.MaskedAccountNumber, &account.AccountHolderName, &account.Currency, &account.TimeZone, &account.Active)
	return account, err
}

func (s Service) CreateAccount(ctx context.Context, input AccountView, actorID, requestID, ip string) (AccountView, error) {
	input = normalizeAccount(input)
	if !validAccount(input) {
		return AccountView{}, ErrInvalidAccount
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return AccountView{}, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `INSERT INTO bank_accounts(account_code,bank_display_name,masked_account_no,account_holder_name,currency,timezone,active,created_by)
		VALUES ($1,$2,$3,$4,'IDR','Asia/Jakarta',true,$5)
		RETURNING id::text,account_code,bank_display_name,masked_account_no,account_holder_name,currency,timezone,active`, input.Code, input.BankDisplayName, input.MaskedAccountNumber, input.AccountHolderName, actorID).
		Scan(&input.ID, &input.Code, &input.BankDisplayName, &input.MaskedAccountNumber, &input.AccountHolderName, &input.Currency, &input.TimeZone, &input.Active)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			return AccountView{}, ErrAccountExists
		}
		return AccountView{}, err
	}
	if err := audit.Append(ctx, tx, actorID, "ACCOUNT_CREATED", "bank_account", input.ID, nil, input, requestID, ip); err != nil {
		return AccountView{}, err
	}
	return input, tx.Commit()
}

func (s Service) UpdateAccount(ctx context.Context, id string, input AccountView, actorID, requestID, ip string) (AccountView, error) {
	input = normalizeAccount(input)
	if !validAccount(input) {
		return AccountView{}, ErrInvalidAccount
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return AccountView{}, err
	}
	defer tx.Rollback()
	var previous AccountView
	err = tx.QueryRowContext(ctx, `SELECT id::text,account_code,bank_display_name,masked_account_no,account_holder_name,currency,timezone,active FROM bank_accounts WHERE id=$1 FOR UPDATE`, id).
		Scan(&previous.ID, &previous.Code, &previous.BankDisplayName, &previous.MaskedAccountNumber, &previous.AccountHolderName, &previous.Currency, &previous.TimeZone, &previous.Active)
	if err != nil {
		return AccountView{}, err
	}
	if previous.Active && !input.Active {
		if err := ensureNoActiveBatches(ctx, tx, id); err != nil {
			return AccountView{}, err
		}
	}
	var updated AccountView
	err = tx.QueryRowContext(ctx, `UPDATE bank_accounts SET account_code=$2,bank_display_name=$3,masked_account_no=$4,account_holder_name=$5,active=$6,updated_at=now()
		WHERE id=$1 RETURNING id::text,account_code,bank_display_name,masked_account_no,account_holder_name,currency,timezone,active`, id, input.Code, input.BankDisplayName, input.MaskedAccountNumber, input.AccountHolderName, input.Active).
		Scan(&updated.ID, &updated.Code, &updated.BankDisplayName, &updated.MaskedAccountNumber, &updated.AccountHolderName, &updated.Currency, &updated.TimeZone, &updated.Active)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			return AccountView{}, ErrAccountExists
		}
		return AccountView{}, err
	}
	if err := audit.Append(ctx, tx, actorID, "ACCOUNT_UPDATED", "bank_account", id, previous, updated, requestID, ip); err != nil {
		return AccountView{}, err
	}
	return updated, tx.Commit()
}

func ensureNoActiveBatches(ctx context.Context, tx *sql.Tx, accountID string) error {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT count(*) FROM reconciliation_batches WHERE account_id=$1 AND state NOT IN ('CLOSED','CANCELLED')`, accountID).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrAccountInUse
	}
	return nil
}

func normalizeAccount(account AccountView) AccountView {
	account.Code = strings.ToUpper(strings.TrimSpace(account.Code))
	account.BankDisplayName = strings.TrimSpace(account.BankDisplayName)
	account.MaskedAccountNumber = strings.TrimSpace(account.MaskedAccountNumber)
	account.AccountHolderName = strings.TrimSpace(account.AccountHolderName)
	account.Currency = CurrencyIDR
	account.TimeZone = "Asia/Jakarta"
	return account
}

func validAccount(account AccountView) bool {
	return account.Code != "" && account.BankDisplayName != "" && account.MaskedAccountNumber != "" && account.AccountHolderName != ""
}
