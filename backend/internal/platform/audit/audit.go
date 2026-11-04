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
	return err
}
