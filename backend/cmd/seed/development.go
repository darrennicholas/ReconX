package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"reconx/backend/internal/modules/banking"
	"reconx/backend/internal/modules/exceptions"
	"reconx/backend/internal/modules/reconciliation"
)

// Development fixtures use dedicated accounts and stable markers so reruns never replace user data.
func seedDevelopment(ctx context.Context, db *sql.DB, adminEmail, storage string) error {
	const requestID = "seed-development"
	var adminID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM users WHERE lower(email)=lower($1)`, adminEmail).Scan(&adminID); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("developer"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	for _, user := range []struct{ name, email, role string }{
		{"ReconX Operator", "operator@reconx.local", "OPERATOR"},
		{"ReconX Checker", "checker@reconx.local", "CHECKER"},
		{"ReconX Auditor", "auditor@reconx.local", "AUDITOR"},
	} {
		_, err = db.ExecContext(ctx, `INSERT INTO users(full_name,email,password_hash,role_id) SELECT $1,$2,$3,id FROM roles WHERE code=$4 AND NOT EXISTS(SELECT 1 FROM users WHERE lower(email)=lower($2))`, user.name, user.email, string(hash), user.role)
		if err != nil {
			return fmt.Errorf("seed user %s: %w", user.email, err)
		}
	}
	var operatorID string
	if err := db.QueryRowContext(ctx, `SELECT u.id::text FROM users u JOIN roles r ON r.id=u.role_id WHERE lower(u.email)='operator@reconx.local' AND r.code='OPERATOR' AND u.active`).Scan(&operatorID); err != nil {
		return fmt.Errorf("seed operator: %w", err)
	}
	bank := banking.Service{DB: db}
	recon := reconciliation.Service{DB: db, StoragePath: storage}
	exc := exceptions.Service{DB: db, StoragePath: storage}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	for _, fixture := range []struct{ code, name, marker, mode string }{
		{"RX-SEED-REVIEW", "Rekening Operasional", "ReconX seed: investigation", "review"},
		{"RX-SEED-APPROVAL", "Rekening Settlement", "ReconX seed: approval", "approval"},
		{"RX-SEED-DRAFT", "Rekening Transaksi", "ReconX seed: draft", "draft"},
	} {
		var accountID string
		err := db.QueryRowContext(ctx, `SELECT id::text FROM bank_accounts WHERE account_code=$1`, fixture.code).Scan(&accountID)
		if err == sql.ErrNoRows {
			account, createErr := bank.CreateAccount(ctx, banking.AccountView{Code: fixture.code, BankDisplayName: fixture.name, MaskedAccountNumber: "**** 1234", AccountHolderName: "ReconX", Active: true}, adminID, requestID, "")
			if createErr != nil {
				return fmt.Errorf("seed account %s: %w", fixture.code, createErr)
			}
			accountID = account.ID
		} else if err != nil {
			return err
		}
		var batchID string
		err = db.QueryRowContext(ctx, `SELECT id::text FROM reconciliation_batches WHERE account_id=$1 AND remarks=$2 ORDER BY created_at LIMIT 1`, accountID, fixture.marker).Scan(&batchID)
		if err == sql.ErrNoRows {
			batch, createErr := recon.CreateBatch(ctx, reconciliation.BatchInput{AccountID: accountID, PeriodFrom: start, PeriodTo: end, Remarks: fixture.marker}, operatorID, requestID, "")
			if createErr != nil {
				return fmt.Errorf("seed batch %s: %w", fixture.marker, createErr)
			}
			batchID = batch.ID
		} else if err != nil {
			return err
		}
		if fixture.mode == "draft" {
			continue
		}
		batch, err := recon.GetBatch(ctx, batchID)
		if err != nil {
			return err
		}
		if batch.State == "DRAFT" {
			imports, err := recon.ListImports(ctx, batchID)
			if err != nil {
				return err
			}
			for _, source := range []string{"INTERNAL", "BANK"} {
				found := false
				for _, item := range imports {
					if item.SourceType == source && item.State == "VALID" && item.SourceVersion == batch.SourceVersion {
						found = true
					}
				}
				if found {
					continue
				}
				date := batch.PeriodFrom.Format("2006-01-02")
				otherDate := date
				if fixture.mode == "review" && source == "BANK" {
					otherDate = batch.PeriodFrom.AddDate(0, 0, 1).Format("2006-01-02")
				}
				csv := fmt.Sprintf("reference_id,date,direction,amount,currency\nRX-001,%s,CREDIT,500000,IDR\nRX-002,%s,DEBIT,150000,IDR\n", date, otherDate)
				item, uploadErr := recon.UploadCSV(ctx, batchID, source, strings.ToLower(source)+"-seed.csv", []byte(csv), operatorID, requestID, "")
				if uploadErr != nil {
					return fmt.Errorf("seed import %s: %w", source, uploadErr)
				}
				if item.State != "VALID" {
					return fmt.Errorf("seed import %s invalid: %v", source, item.Errors)
				}
			}
			batch, err = recon.GetBatch(ctx, batchID)
			if err != nil {
				return err
			}
		}
		if batch.State == "FAILED" {
			if err = recon.RetryFailedBatch(ctx, batchID, batch.RowVersion, operatorID, requestID, ""); err != nil {
				return fmt.Errorf("retry seed matching: %w", err)
			}
			batch, err = recon.GetBatch(ctx, batchID)
			if err != nil {
				return err
			}
		}
		if batch.State == "READY" {
			if _, err = recon.RunMatching(ctx, batchID, batch.RowVersion, operatorID, requestID, ""); err != nil {
				return fmt.Errorf("seed matching: %w", err)
			}
			batch, err = recon.GetBatch(ctx, batchID)
			if err != nil {
				return err
			}
		}
		if batch.State == "REVIEW" && fixture.mode == "approval" {
			if _, err = recon.SubmitBatch(ctx, batchID, batch.RowVersion, operatorID, requestID, ""); err != nil {
				return fmt.Errorf("seed batch approval: %w", err)
			}
		}
		if batch.State == "REVIEW" && fixture.mode == "review" {
			var exceptionID string
			err = db.QueryRowContext(ctx, `SELECT id::text FROM reconciliation_exceptions WHERE batch_id=$1 AND type='DATE_VARIANCE' AND state='OPEN' LIMIT 1`, batchID).Scan(&exceptionID)
			if err == nil {
				if _, err = exc.ProposeManualMatch(ctx, exceptionID, "Tanggal berbeda satu hari; periksa bukti transaksi sebelum persetujuan.", operatorID, requestID, ""); err != nil {
					return fmt.Errorf("seed proposal: %w", err)
				}
			} else if err != sql.ErrNoRows {
				return err
			}
		}
	}
	for _, email := range []string{adminEmail, "operator@reconx.local", "checker@reconx.local", "auditor@reconx.local"} {
		_, err = db.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,target_entity,target_entity_id) SELECT id,'SEED_READY','Workspace siap','Data awal rekonsiliasi tersedia.','','' FROM users u WHERE lower(email)=lower($1) AND NOT EXISTS(SELECT 1 FROM notifications n WHERE n.user_id=u.id AND n.type='SEED_READY')`, email)
		if err != nil {
			return err
		}
	}
	return nil
}
