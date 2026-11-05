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
		{"Operator Cabang", "operator.cabang@reconx.local", "OPERATOR"},
		{"Checker Cadangan", "checker.cadangan@reconx.local", "CHECKER"},
		{"Auditor Internal", "auditor.internal@reconx.local", "AUDITOR"},
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
	var checkerID string
	if err := db.QueryRowContext(ctx, `SELECT u.id::text FROM users u JOIN roles r ON r.id=u.role_id WHERE lower(u.email)='checker@reconx.local' AND r.code='CHECKER' AND u.active`).Scan(&checkerID); err != nil {
		return fmt.Errorf("seed checker: %w", err)
	}
	var branchOperatorID string
	if err := db.QueryRowContext(ctx, `SELECT u.id::text FROM users u JOIN roles r ON r.id=u.role_id WHERE lower(u.email)='operator.cabang@reconx.local' AND r.code='OPERATOR' AND u.active`).Scan(&branchOperatorID); err != nil {
		return fmt.Errorf("seed branch operator: %w", err)
	}
	bank := banking.Service{DB: db}
	recon := reconciliation.Service{DB: db, StoragePath: storage}
	exc := exceptions.Service{DB: db, StoragePath: storage}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for _, fixture := range []struct {
		code, name, marker, mode string
		monthsAgo                int
	}{
		{"RX-SEED-REVIEW", "Rekening Operasional", "ReconX seed: investigation", "review", 0},
		{"RX-SEED-APPROVAL", "Rekening Settlement", "ReconX seed: approval", "approval", 0},
		{"RX-SEED-DRAFT", "Rekening Transaksi", "ReconX seed: draft", "draft", 0},
		{"RX-SEED-QRIS", "Settlement QRIS", "ReconX seed: QRIS", "mixed", 0},
		{"RX-SEED-TRANSFER", "Transfer Antarbank", "ReconX seed: transfer", "mixed", 1},
		{"RX-SEED-EDC", "Settlement EDC", "ReconX seed: EDC", "mixed", 2},
		{"RX-SEED-VA", "Virtual Account", "ReconX seed: virtual account", "approval", 1},
		{"RX-SEED-CLOSED1", "Rekening Giro", "ReconX seed: closed giro", "closed", 1},
		{"RX-SEED-CLOSED2", "Rekening Cabang", "ReconX seed: closed cabang", "closed", 2},
		{"RX-SEED-READY", "Rekening Payment", "ReconX seed: ready", "ready", 0},
		{"RX-SEED-NEW", "Rekening Koleksi", "ReconX seed: draft koleksi", "draft", 0},
		{"RX-EX-DIRECTION", "Rekening Arah Transaksi", "ReconX seed: arah berbeda", "direction", 0},
		{"RX-EX-INVESTIGATE", "Rekening Investigasi", "ReconX seed: investigasi aktif", "investigate", 0},
		{"RX-EX-MANUAL-OK", "Rekening Manual Match", "ReconX seed: manual disetujui", "manual-approved", 0},
		{"RX-EX-MANUAL-NO", "Rekening Manual Review", "ReconX seed: manual ditolak", "manual-rejected", 0},
		{"RX-BATCH-APPROVED", "Rekening Siap Tutup", "ReconX seed: batch disetujui", "approved", 0},
		{"RX-BATCH-REJECT", "Rekening Persetujuan Ditolak", "ReconX seed: batch ditolak", "rejected", 0},
		{"RX-BATCH-CANCEL", "Rekening Batch Dibatalkan", "ReconX seed: batch dibatalkan", "cancelled", 0},
		{"RX-IMPORT-INVALID", "Rekening Validasi Import", "ReconX seed: import gagal", "invalid-import", 0},
		{"RX-EX-AMBIGUOUS", "Rekening Referensi Ganda", "ReconX seed: referensi ganda", "ambiguous", 0},
	} {
		periodStart := start.AddDate(0, -fixture.monthsAgo, 0)
		periodEnd := periodStart.AddDate(0, 1, -1)
		makerID := operatorID
		if fixture.code == "RX-SEED-EDC" || fixture.code == "RX-SEED-CLOSED2" || fixture.code == "RX-SEED-NEW" {
			makerID = branchOperatorID
		}
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
			batch, createErr := recon.CreateBatch(ctx, reconciliation.BatchInput{AccountID: accountID, PeriodFrom: periodStart, PeriodTo: periodEnd, Remarks: fixture.marker}, makerID, requestID, "")
			if createErr != nil {
				return fmt.Errorf("seed batch %s: %w", fixture.marker, createErr)
			}
			batchID = batch.ID
		} else if err != nil {
			return err
		}
		if fixture.mode == "cancelled" {
			batch, getErr := recon.GetBatch(ctx, batchID)
			if getErr != nil {
				return getErr
			}
			if batch.State == "DRAFT" {
				if err := recon.CancelDraft(ctx, batchID, batch.RowVersion, makerID, requestID, ""); err != nil {
					return fmt.Errorf("seed cancelled batch: %w", err)
				}
			}
			continue
		}
		if fixture.mode == "invalid-import" {
			imports, listErr := recon.ListImports(ctx, batchID)
			if listErr != nil {
				return listErr
			}
			if len(imports) == 0 {
				item, uploadErr := recon.UploadCSV(ctx, batchID, "INTERNAL", "internal-invalid-seed.csv", []byte("reference_id,date,direction,amount,currency\nINVALID-001,invalid,CREDIT,50000,IDR\n"), makerID, requestID, "")
				if uploadErr != nil {
					return fmt.Errorf("seed invalid import: %w", uploadErr)
				}
				if item.State != "INVALID" {
					return fmt.Errorf("expected invalid import, got %s", item.State)
				}
			}
			continue
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
				if (fixture.mode == "review" || fixture.mode == "manual-approved" || fixture.mode == "manual-rejected") && source == "BANK" {
					otherDate = batch.PeriodFrom.AddDate(0, 0, 1).Format("2006-01-02")
				}
				csv := fmt.Sprintf("reference_id,date,direction,amount,currency\nRX-001,%s,CREDIT,500000,IDR\nRX-002,%s,DEBIT,150000,IDR\n", date, otherDate)
				if fixture.mode == "ambiguous" {
					csv = fmt.Sprintf("reference_id,date,direction,amount,currency\nAMB-001,%s,CREDIT,100000,IDR\nAMB-002,%s,DEBIT,150000,IDR\n", date, date)
					if source == "INTERNAL" {
						csv += fmt.Sprintf("AMB-001,%s,CREDIT,100000,IDR\n", date)
					}
				}
				if fixture.mode == "direction" && source == "BANK" {
					csv = fmt.Sprintf("reference_id,date,direction,amount,currency\nRX-001,%s,DEBIT,500000,IDR\nRX-002,%s,DEBIT,150000,IDR\n", date, date)
				}
				if fixture.mode == "investigate" && source == "BANK" {
					csv = fmt.Sprintf("reference_id,date,direction,amount,currency\nRX-001,%s,CREDIT,510000,IDR\nRX-002,%s,DEBIT,150000,IDR\n", date, date)
				}
				if strings.HasPrefix(fixture.code, "RX-SEED-") && fixture.code != "RX-SEED-REVIEW" && fixture.code != "RX-SEED-APPROVAL" {
					var rows strings.Builder
					rows.WriteString("reference_id,date,direction,amount,currency\n")
					for i := 1; i <= 20; i++ {
						amount := 50000 + i*25000
						if fixture.mode == "mixed" && source == "BANK" && i == 18 {
							amount += 10000
						}
						if fixture.mode == "mixed" && source == "BANK" && i == 19 {
							continue
						}
						fmt.Fprintf(&rows, "RX-%03d,%s,DEBIT,%d,IDR\n", i, batch.PeriodFrom.AddDate(0, 0, (i-1)%20).Format("2006-01-02"), amount)
					}
					if fixture.mode == "mixed" && source == "BANK" {
						fmt.Fprintf(&rows, "RX-BANK-ONLY,%s,CREDIT,75000,IDR\n", date)
					}
					csv = rows.String()
				}
				item, uploadErr := recon.UploadCSV(ctx, batchID, source, strings.ToLower(source)+"-seed.csv", []byte(csv), makerID, requestID, "")
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
			if err = recon.RetryFailedBatch(ctx, batchID, batch.RowVersion, makerID, requestID, ""); err != nil {
				return fmt.Errorf("retry seed matching: %w", err)
			}
			batch, err = recon.GetBatch(ctx, batchID)
			if err != nil {
				return err
			}
		}
		if batch.State == "READY" && fixture.mode != "ready" {
			if _, err = recon.RunMatching(ctx, batchID, batch.RowVersion, makerID, requestID, ""); err != nil {
				return fmt.Errorf("seed matching: %w", err)
			}
			batch, err = recon.GetBatch(ctx, batchID)
			if err != nil {
				return err
			}
		}
		if batch.State == "REVIEW" && (fixture.mode == "approval" || fixture.mode == "closed" || fixture.mode == "approved" || fixture.mode == "rejected") {
			var previous bool
			if err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM approval_requests WHERE batch_id=$1 AND kind='BATCH_CLOSE')`, batchID).Scan(&previous); err != nil {
				return err
			}
			if previous && fixture.mode == "rejected" {
				continue
			}
			if _, err = recon.SubmitBatch(ctx, batchID, batch.RowVersion, makerID, requestID, ""); err != nil {
				return fmt.Errorf("seed batch approval: %w", err)
			}
		}
		if fixture.mode == "approved" || fixture.mode == "rejected" {
			batch, err = recon.GetBatch(ctx, batchID)
			if err != nil {
				return err
			}
			if batch.State == "PENDING_APPROVAL" {
				var approvalID string
				if err = db.QueryRowContext(ctx, `SELECT id::text FROM approval_requests WHERE batch_id=$1 AND kind='BATCH_CLOSE' AND state='PENDING' ORDER BY created_at DESC LIMIT 1`, batchID).Scan(&approvalID); err != nil {
					return err
				}
				decision := "APPROVE"
				if fixture.mode == "rejected" {
					decision = "REJECT"
				}
				if err = exc.DecideApproval(ctx, approvalID, decision, "Hasil pemeriksaan persetujuan batch.", checkerID, requestID, ""); err != nil {
					return fmt.Errorf("seed batch decision: %w", err)
				}
			}
		}
		if fixture.mode == "closed" {
			batch, err = recon.GetBatch(ctx, batchID)
			if err != nil {
				return err
			}
			if batch.State == "PENDING_APPROVAL" {
				var approvalID string
				if err = db.QueryRowContext(ctx, `SELECT id::text FROM approval_requests WHERE batch_id=$1 AND kind='BATCH_CLOSE' AND state='PENDING' ORDER BY created_at DESC LIMIT 1`, batchID).Scan(&approvalID); err != nil {
					return err
				}
				if err = exc.DecideApproval(ctx, approvalID, "APPROVE", "Pemeriksaan batch selesai.", checkerID, requestID, ""); err != nil {
					return fmt.Errorf("seed approval: %w", err)
				}
				batch, err = recon.GetBatch(ctx, batchID)
				if err != nil {
					return err
				}
			}
			if batch.State == "APPROVED" {
				if err = exc.CloseApprovedBatch(ctx, batchID, batch.RowVersion, checkerID, requestID, ""); err != nil {
					return fmt.Errorf("seed close: %w", err)
				}
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
		if batch.State == "REVIEW" && (fixture.mode == "manual-approved" || fixture.mode == "manual-rejected") {
			var exceptionID string
			err = db.QueryRowContext(ctx, `SELECT e.id::text FROM reconciliation_exceptions e WHERE e.batch_id=$1 AND e.type='DATE_VARIANCE' AND e.state='OPEN' AND NOT EXISTS (SELECT 1 FROM approval_requests a WHERE a.exception_id=e.id) LIMIT 1`, batchID).Scan(&exceptionID)
			if err == nil {
				proposal, proposalErr := exc.ProposeManualMatch(ctx, exceptionID, "Perbedaan tanggal diperiksa menggunakan bukti transaksi.", makerID, requestID, "")
				if proposalErr != nil {
					return fmt.Errorf("seed manual proposal: %w", proposalErr)
				}
				decision := "APPROVE"
				if fixture.mode == "manual-rejected" {
					decision = "REJECT"
				}
				if err = exc.DecideApproval(ctx, proposal.ID, decision, "Hasil pemeriksaan manual match.", checkerID, requestID, ""); err != nil {
					return fmt.Errorf("seed manual decision: %w", err)
				}
			} else if err != sql.ErrNoRows {
				return err
			}
		}
		if batch.State == "REVIEW" && fixture.mode == "investigate" {
			var exceptionID string
			err = db.QueryRowContext(ctx, `SELECT id::text FROM reconciliation_exceptions WHERE batch_id=$1 AND type='AMOUNT_MISMATCH' AND state='OPEN' LIMIT 1`, batchID).Scan(&exceptionID)
			if err == nil {
				if _, err = exc.AddAction(ctx, exceptionID, exceptions.ActionInput{Type: "ASSIGN", AssigneeID: branchOperatorID, Note: "Periksa perbedaan nominal pada sumber transaksi."}, makerID, requestID, ""); err != nil {
					return fmt.Errorf("seed assignment: %w", err)
				}
			} else if err != sql.ErrNoRows {
				return err
			}
		}
	}
	for _, email := range []string{adminEmail, "operator@reconx.local", "checker@reconx.local", "auditor@reconx.local", "operator.cabang@reconx.local", "checker.cadangan@reconx.local", "auditor.internal@reconx.local"} {
		_, err = db.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,target_entity,target_entity_id) SELECT id,'SEED_READY','Workspace siap','Data awal rekonsiliasi tersedia.','','' FROM users u WHERE lower(email)=lower($1) AND NOT EXISTS(SELECT 1 FROM notifications n WHERE n.user_id=u.id AND n.type='SEED_READY')`, email)
		if err != nil {
			return err
		}
	}
	return nil
}
