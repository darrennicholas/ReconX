package audit

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestActorNotification(t *testing.T) {
	tests := []struct {
		name, action, entity, id      string
		after                         any
		wantTitle, wantTarget, wantID string
	}{
		{"batch", "BATCH_CREATED", "batch", "batch-1", nil, "Batch dibuat", "batch", "batch-1"},
		{"valid import", "IMPORT_VALIDATED", "import", "import-1", map[string]any{"state": "VALID", "batch_id": "batch-1"}, "Import CSV berhasil", "batch", "batch-1"},
		{"invalid import already notified", "IMPORT_VALIDATED", "import", "import-1", map[string]any{"state": "INVALID"}, "", "", ""},
		{"matching already notified", "MATCH_RUN_COMPLETED", "run", "run-1", nil, "", "", ""},
		{"manual proposal", "MANUAL_MATCH_PROPOSED", "approval", "approval-1", map[string]any{"exception_id": "exception-1"}, "Manual match diajukan", "exception", "exception-1"},
		{"login ignored", "LOGIN", "user", "user-1", nil, "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, target, id := actorNotification(tt.action, tt.entity, tt.id, tt.after)
			if title != tt.wantTitle || target != tt.wantTarget || id != tt.wantID {
				t.Fatalf("got (%q, %q, %q), want (%q, %q, %q)", title, target, id, tt.wantTitle, tt.wantTarget, tt.wantID)
			}
		})
	}
}

func TestActorNotificationTransaction(t *testing.T) {
	url := os.Getenv("RECONX_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set RECONX_TEST_DATABASE_URL to run the PostgreSQL integration check")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var actor string
	if err := tx.QueryRowContext(ctx, `SELECT id::text FROM users WHERE email='admin@reconx.local'`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if err := Append(ctx, tx, actor, "BATCH_CREATED", "batch", "notification-test", nil, nil, "notification-test", ""); err != nil {
		t.Fatal(err)
	}
	var count, other int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE user_id=$1), count(*) FILTER (WHERE user_id<>$1) FROM notifications WHERE type='ACTOR_BATCH_CREATED' AND target_entity_id='notification-test'`, actor).Scan(&count, &other); err != nil {
		t.Fatal(err)
	}
	if count != 1 || other != 0 {
		t.Fatalf("expected one notification for actor only; got actor=%d others=%d", count, other)
	}
}
