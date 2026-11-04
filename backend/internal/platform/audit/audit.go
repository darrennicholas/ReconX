package audit

import (
	"context"
	"database/sql"
	"encoding/json"
)

// Append writes an immutable event inside the caller's transaction.
func Append(ctx context.Context, tx *sql.Tx, actorID, action, entity, entityID string, before, after any, requestID, ip string) error {
	var beforeJSON, afterJSON any
	if before != nil {
		value, err := json.Marshal(before)
		if err != nil {
			return err
		}
		beforeJSON = string(value)
	}
	if after != nil {
		value, err := json.Marshal(after)
		if err != nil {
			return err
		}
		afterJSON = string(value)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_logs(actor_id,action,entity,entity_id,before_json,after_json,request_id,ip_address)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7,NULLIF($8,'')::inet)`, actorID, action, entity, entityID, beforeJSON, afterJSON, requestID, ip)
	if err != nil {
		return err
	}
	title, target, targetID := actorNotification(action, entity, entityID, after)
	if title == "" || actorID == "" {
		return nil
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,target_entity,target_entity_id)
		VALUES ($1,$2,$3,$4,$5,$6)`, actorID, "ACTOR_"+action, title, "Tindakan Anda berhasil disimpan.", target, targetID)
	return err
}

// actorNotification only covers completed business actions. Existing matching and
// failed-import notifications already target the actor and must not be duplicated.
func actorNotification(action, entity, entityID string, after any) (title, target, targetID string) {
	titles := map[string]string{
		"USER_CREATED": "Pengguna dibuat", "USER_UPDATED": "Pengguna diperbarui", "USER_PASSWORD_RESET": "Password pengguna direset",
		"ACCOUNT_CREATED": "Rekening dibuat", "ACCOUNT_UPDATED": "Rekening diperbarui",
		"BATCH_CREATED": "Batch dibuat", "BATCH_UPDATED": "Batch diperbarui", "BATCH_CANCELLED": "Batch dibatalkan",
		"BATCH_REOPENED": "Batch dibuka kembali", "BATCH_SUBMITTED": "Batch diajukan untuk approval", "BATCH_CLOSED": "Batch ditutup",
		"MATCH_RUN_RETRY": "Matching siap dicoba kembali", "MATCH_RUN_FAILED": "Matching gagal",
		"EXCEPTION_ASSIGN": "Exception ditugaskan", "EXCEPTION_NOTE": "Catatan exception disimpan", "EXCEPTION_EVIDENCE_UPLOADED": "Evidence ditambahkan",
		"MANUAL_MATCH_PROPOSED": "Manual match diajukan", "APPROVAL_APPROVED": "Pengajuan disetujui", "APPROVAL_REJECTED": "Pengajuan ditolak",
		"SETTINGS_UPDATED": "Pengaturan diperbarui",
	}
	if action == "IMPORT_VALIDATED" {
		values, ok := after.(map[string]any)
		if !ok || values["state"] != "VALID" {
			return "", "", ""
		}
		batchID, _ := values["batch_id"].(string)
		return "Import CSV berhasil", "batch", batchID
	}
	title = titles[action]
	if title == "" {
		return "", "", ""
	}
	switch entity {
	case "batch", "exception", "approval", "bank_account", "user", "settings":
		target, targetID = entity, entityID
	}
	if action == "MANUAL_MATCH_PROPOSED" {
		if values, ok := after.(map[string]any); ok {
			if id, ok := values["exception_id"].(string); ok {
				target, targetID = "exception", id
			}
		}
	}
	return title, target, targetID
}
