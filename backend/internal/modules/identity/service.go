package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"reconx/backend/internal/platform/audit"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailInUse         = errors.New("email already exists")
	ErrLastActiveAdmin    = errors.New("cannot disable the last active admin")
	ErrInvalidRole        = errors.New("invalid role")
	ErrWeakPassword       = errors.New("password must contain at least 12 characters")
)

const SessionCookieName = "reconx_session"
const CSRFCookieName = "reconx_csrf"
const SessionDuration = 12 * time.Hour

type Service struct {
	DB *sql.DB
}

type User struct {
	ID        string    `json:"id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type Credentials struct {
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
	User         User
}

// SeedBootstrapAdmin creates the first user only when the users table is empty.
func SeedBootstrapAdmin(ctx context.Context, db *sql.DB, fullName, email, password string) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	fullName = strings.TrimSpace(fullName)
	email = strings.ToLower(strings.TrimSpace(email))
	if fullName == "" || !validEmail(email) {
		return errors.New("bootstrap admin name and email are required")
	}
	var hash string
	var err error
	if password == "developer" {
		var encoded []byte
		encoded, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		hash = string(encoded)
	} else {
		hash, err = HashPassword(password)
	}
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO users(full_name,email,password_hash,role_id)
		SELECT $1,$2,$3,id FROM roles WHERE code='ADMIN'`, fullName, email, hash)
	return err
}

func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func (s Service) Login(ctx context.Context, email, password, requestID, ip string) (Credentials, error) {
	var user User
	var passwordHash string
	err := s.DB.QueryRowContext(ctx, `SELECT u.id::text, u.full_name, u.email, r.code, u.active, u.created_at, u.password_hash
		FROM users u JOIN roles r ON r.id = u.role_id WHERE lower(u.email) = lower($1)`, strings.TrimSpace(email)).
		Scan(&user.ID, &user.FullName, &user.Email, &user.Role, &user.Active, &user.CreatedAt, &passwordHash)
	if err != nil || !user.Active || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		_, _ = s.DB.ExecContext(ctx, `INSERT INTO audit_logs(actor_id, action, entity, entity_id, after_json, request_id, ip_address)
			VALUES (NULL, 'LOGIN_FAILED', 'auth', $1, $2::jsonb, $3, NULLIF($4,'')::inet)`, strings.ToLower(strings.TrimSpace(email)), `{"reason":"invalid_credentials"}`, requestID, ip)
		return Credentials{}, ErrInvalidCredentials
	}

	token, err := randomToken(32)
	if err != nil {
		return Credentials{}, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return Credentials{}, err
	}
	expiresAt := time.Now().UTC().Add(SessionDuration)
	tokenHash := sha256.Sum256([]byte(token))
	csrfHash := sha256.Sum256([]byte(csrf))

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Credentials{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash, csrf_token_hash, user_id, expires_at) VALUES ($1,$2,$3,$4)`, tokenHash[:], csrfHash[:], user.ID, expiresAt); err != nil {
		return Credentials{}, err
	}
	if err := audit.Append(ctx, tx, user.ID, "LOGIN", "user", user.ID, nil, map[string]string{"result": "success"}, requestID, ip); err != nil {
		return Credentials{}, err
	}
	if err := tx.Commit(); err != nil {
		return Credentials{}, err
	}
	return Credentials{SessionToken: token, CSRFToken: csrf, ExpiresAt: expiresAt, User: user}, nil
}

func (s Service) Authenticate(ctx context.Context, token string) (Principal, string, []byte, error) {
	if token == "" {
		return Principal{}, "", nil, ErrUnauthenticated
	}
	hash := sha256.Sum256([]byte(token))
	var principal Principal
	var sessionID string
	var csrfHash []byte
	err := s.DB.QueryRowContext(ctx, `SELECT u.id::text, r.code, u.active, s.id::text, s.csrf_token_hash
		FROM sessions s JOIN users u ON u.id = s.user_id JOIN roles r ON r.id = u.role_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now()`, hash[:]).
		Scan(&principal.UserID, &principal.Role, &principal.IsActive, &sessionID, &csrfHash)
	if err != nil || !principal.IsActive || !IsRole(principal.Role) {
		return Principal{}, "", nil, ErrUnauthenticated
	}
	_, _ = s.DB.ExecContext(ctx, `UPDATE sessions SET last_seen_at = now() WHERE id = $1`, sessionID)
	return principal, sessionID, csrfHash, nil
}

func (s Service) CSRFHash(ctx context.Context, sessionID string) ([]byte, error) {
	var hash []byte
	err := s.DB.QueryRowContext(ctx, `SELECT csrf_token_hash FROM sessions WHERE id = $1 AND revoked_at IS NULL AND expires_at > now()`, sessionID).Scan(&hash)
	return hash, err
}

func (s Service) Logout(ctx context.Context, sessionID, userID, requestID, ip string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`, sessionID, userID); err != nil {
		return err
	}
	if err := audit.Append(ctx, tx, userID, "LOGOUT", "user", userID, nil, nil, requestID, ip); err != nil {
		return err
	}
	return tx.Commit()
}

func (s Service) Me(ctx context.Context, userID string) (User, error) {
	var user User
	err := s.DB.QueryRowContext(ctx, `SELECT u.id::text, u.full_name, u.email, r.code, u.active, u.created_at
		FROM users u JOIN roles r ON r.id = u.role_id WHERE u.id = $1 AND u.active`, userID).
		Scan(&user.ID, &user.FullName, &user.Email, &user.Role, &user.Active, &user.CreatedAt)
	return user, err
}

func (s Service) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT u.id::text, u.full_name, u.email, r.code, u.active, u.created_at
		FROM users u JOIN roles r ON r.id = u.role_id ORDER BY u.full_name, u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.FullName, &user.Email, &user.Role, &user.Active, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s Service) ListUsersPage(ctx context.Context, query, role, state string, page, pageSize int) ([]User, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}
	base := ` FROM users u JOIN roles r ON r.id=u.role_id WHERE ($1='' OR u.full_name ILIKE '%'||$1||'%' OR u.email ILIKE '%'||$1||'%') AND ($2='' OR r.code=$2) AND ($3='' OR ($3='ACTIVE' AND u.active) OR ($3='INACTIVE' AND NOT u.active))`
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*)`+base, strings.TrimSpace(query), strings.ToUpper(role), strings.ToUpper(state)).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT u.id::text,u.full_name,u.email,r.code,u.active,u.created_at`+base+` ORDER BY u.full_name,u.id LIMIT $4 OFFSET $5`, strings.TrimSpace(query), strings.ToUpper(role), strings.ToUpper(state), pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]User, 0)
	for rows.Next() {
		var item User
		if err := rows.Scan(&item.ID, &item.FullName, &item.Email, &item.Role, &item.Active, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s Service) CreateUser(ctx context.Context, fullName, email, password string, role Role, actorID, requestID, ip string) (User, error) {
	fullName = strings.TrimSpace(fullName)
	email = strings.ToLower(strings.TrimSpace(email))
	if fullName == "" || !validEmail(email) || !IsRole(role) {
		return User{}, ErrInvalidRole
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	var user User
	err = tx.QueryRowContext(ctx, `INSERT INTO users(full_name,email,password_hash,role_id)
		SELECT $1,$2,$3,id FROM roles WHERE code = $4
		RETURNING id::text, full_name, email, active, created_at`, fullName, email, hash, role).
		Scan(&user.ID, &user.FullName, &user.Email, &user.Active, &user.CreatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			return User{}, ErrEmailInUse
		}
		return User{}, err
	}
	user.Role = role
	if err := audit.Append(ctx, tx, actorID, "USER_CREATED", "user", user.ID, nil, map[string]any{"email": user.Email, "role": user.Role, "active": true}, requestID, ip); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s Service) UpdateUser(ctx context.Context, targetID, fullName, email string, role Role, active bool, actorID, requestID, ip string) (User, error) {
	fullName = strings.TrimSpace(fullName)
	email = strings.ToLower(strings.TrimSpace(email))
	if fullName == "" || !validEmail(email) || !IsRole(role) {
		return User{}, ErrInvalidRole
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(884212)`); err != nil {
		return User{}, err
	}
	var previous User
	err = tx.QueryRowContext(ctx, `SELECT u.id::text, u.full_name, u.email, r.code, u.active, u.created_at FROM users u JOIN roles r ON r.id = u.role_id WHERE u.id = $1 FOR UPDATE OF u`, targetID).
		Scan(&previous.ID, &previous.FullName, &previous.Email, &previous.Role, &previous.Active, &previous.CreatedAt)
	if err != nil {
		return User{}, err
	}
	if previous.Active && previous.Role == RoleAdmin && (!active || role != RoleAdmin) {
		var remaining int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users u JOIN roles r ON r.id=u.role_id WHERE u.active AND r.code='ADMIN' AND u.id<>$1`, targetID).Scan(&remaining); err != nil {
			return User{}, err
		}
		if remaining == 0 {
			return User{}, ErrLastActiveAdmin
		}
	}
	var updated User
	err = tx.QueryRowContext(ctx, `UPDATE users u SET full_name=$2,email=$3,role_id=r.id,active=$4,updated_at=now()
		FROM roles r WHERE u.id=$1 AND r.code=$5
		RETURNING u.id::text,u.full_name,u.email,r.code,u.active,u.created_at`, targetID, fullName, email, active, role).
		Scan(&updated.ID, &updated.FullName, &updated.Email, &updated.Role, &updated.Active, &updated.CreatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			return User{}, ErrEmailInUse
		}
		return User{}, err
	}
	if previous.Active && !updated.Active {
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, targetID); err != nil {
			return User{}, err
		}
	}
	if err := audit.Append(ctx, tx, actorID, "USER_UPDATED", "user", updated.ID, map[string]any{"email": previous.Email, "role": previous.Role, "active": previous.Active}, map[string]any{"email": updated.Email, "role": updated.Role, "active": updated.Active}, requestID, ip); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return updated, nil
}

func (s Service) ResetPassword(ctx context.Context, targetID, password, actorID, requestID, ip string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=$2,updated_at=now() WHERE id=$1`, targetID, hash)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, targetID); err != nil {
		return err
	}
	if err := audit.Append(ctx, tx, actorID, "USER_PASSWORD_RESET", "user", targetID, nil, nil, requestID, ip); err != nil {
		return err
	}
	return tx.Commit()
}

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && strings.Contains(value, "@")
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
