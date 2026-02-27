package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var JwtSecret []byte
var TokenExpiry time.Duration = 24 * time.Hour

func init() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		secret = hex.EncodeToString(b)
		fmt.Fprintf(os.Stderr, "⚠️  JWT_SECRET not set. Auto-generated for this session (tokens won't survive restart).\n")
	}
	JwtSecret = []byte(secret)

	if exp := os.Getenv("JWT_EXPIRY_HOURS"); exp != "" {
		if h, err := strconv.Atoi(exp); err == nil && h > 0 {
			TokenExpiry = time.Duration(h) * time.Hour
		}
	}
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type UserTenant struct {
	TenantID   string `json:"tenant_id"`
	TenantName string `json:"tenant_name"`
	Role       string `json:"role"`
}

func SetupAuthTables(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tenants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL UNIQUE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS user_tenants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			role VARCHAR(20) NOT NULL DEFAULT 'member',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, tenant_id)
		);

		CREATE TABLE IF NOT EXISTS collection_permissions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			table_name TEXT NOT NULL,
			can_read BOOLEAN DEFAULT false,
			can_create BOOLEAN DEFAULT false,
			can_update BOOLEAN DEFAULT false,
			can_delete BOOLEAN DEFAULT false,
			UNIQUE(user_id, tenant_id, table_name)
		);

		-- Audit Logs table
		CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			user_email TEXT NOT NULL,
			table_name TEXT NOT NULL,
			action TEXT NOT NULL,
			payload JSONB DEFAULT '{}' NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
		);
		ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
		DROP POLICY IF EXISTS tenant_audit_log_policy ON audit_logs;
		CREATE POLICY tenant_audit_log_policy ON audit_logs
			USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
	`)
	return err
}

// RegisterNewTenant creates a new tenant and registers the user as admin.
func RegisterNewTenant(ctx context.Context, pool *pgxpool.Pool, email, password, tenantName string) (*User, *UserTenant, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	// Create tenant
	var tenantID string
	err = tx.QueryRow(ctx, "INSERT INTO tenants (name) VALUES ($1) RETURNING id", tenantName).Scan(&tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("tenant name already exists")
	}

	// Create or get user (update password on conflict so re-registrations work)
	var userID string
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) 
		 ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash
		 RETURNING id`,
		email, string(hash),
	).Scan(&userID)
	if err != nil {
		return nil, nil, err
	}

	// Link user to tenant as admin
	_, err = tx.Exec(ctx,
		"INSERT INTO user_tenants (user_id, tenant_id, role) VALUES ($1, $2, 'admin')",
		userID, tenantID,
	)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return &User{ID: userID, Email: email},
		&UserTenant{TenantID: tenantID, TenantName: tenantName, Role: "admin"}, nil
}

// RegisterJoinTenant registers a user into an existing tenant as member (0 permissions).
func RegisterJoinTenant(ctx context.Context, pool *pgxpool.Pool, email, password, tenantName string) (*User, *UserTenant, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, err
	}

	// Find tenant by name
	var tenantID string
	err = pool.QueryRow(ctx, "SELECT id FROM tenants WHERE name = $1", tenantName).Scan(&tenantID)
	if err != nil {
		return nil, nil, errors.New("tenant not found")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	// Create or get user (update password on conflict so re-registrations work)
	var userID string
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) 
		 ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash
		 RETURNING id`,
		email, string(hash),
	).Scan(&userID)
	if err != nil {
		return nil, nil, err
	}

	// Link user to tenant as member
	_, err = tx.Exec(ctx,
		"INSERT INTO user_tenants (user_id, tenant_id, role) VALUES ($1, $2, 'member') ON CONFLICT DO NOTHING",
		userID, tenantID,
	)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return &User{ID: userID, Email: email},
		&UserTenant{TenantID: tenantID, TenantName: tenantName, Role: "member"}, nil
}

// Login authenticates a user and returns user info + list of tenants.
func Login(ctx context.Context, pool *pgxpool.Pool, email, password string) (*User, []UserTenant, error) {
	var userID, hash string
	err := pool.QueryRow(ctx, "SELECT id, password_hash FROM users WHERE email = $1", email).
		Scan(&userID, &hash)
	if err != nil {
		return nil, nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, nil, errors.New("invalid credentials")
	}

	tenants, err := GetUserTenants(ctx, pool, userID)
	if err != nil {
		return nil, nil, err
	}

	return &User{ID: userID, Email: email}, tenants, nil
}

// GetUserTenants returns all tenants a user belongs to.
func GetUserTenants(ctx context.Context, pool *pgxpool.Pool, userID string) ([]UserTenant, error) {
	rows, err := pool.Query(ctx, `
		SELECT t.id, t.name, ut.role
		FROM user_tenants ut
		JOIN tenants t ON t.id = ut.tenant_id
		WHERE ut.user_id = $1
		ORDER BY t.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []UserTenant
	for rows.Next() {
		var ut UserTenant
		if err := rows.Scan(&ut.TenantID, &ut.TenantName, &ut.Role); err != nil {
			continue
		}
		tenants = append(tenants, ut)
	}
	return tenants, nil
}

// GenerateAccessToken creates a signed JWT with tenant and role context.
func GenerateAccessToken(user *User, tenantID, role string) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       user.ID,
		"email":     user.Email,
		"tenant_id": tenantID,
		"role":      role,
		"iat":       now.Unix(),
		"nbf":       now.Unix(),
		"exp":       now.Add(TokenExpiry).Unix(),
	})
	return token.SignedString(JwtSecret)
}

// ValidateToken parses and validates a JWT string.
func ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return JwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

// CheckPermission verifies if user has the specified CRUD action on a collection.
func CheckPermission(ctx context.Context, pool *pgxpool.Pool, userID, tenantID, tableName, action string) (bool, error) {
	// First check if user is admin (admin bypasses all)
	var role string
	err := pool.QueryRow(ctx,
		"SELECT role FROM user_tenants WHERE user_id = $1 AND tenant_id = $2",
		userID, tenantID,
	).Scan(&role)
	if err != nil {
		return false, errors.New("user not in tenant")
	}
	if role == "admin" {
		return true, nil
	}

	// Check specific permission
	var colName string
	switch action {
	case "read":
		colName = "can_read"
	case "create":
		colName = "can_create"
	case "update":
		colName = "can_update"
	case "delete":
		colName = "can_delete"
	default:
		return false, nil
	}

	var allowed bool
	query := fmt.Sprintf(
		"SELECT %s FROM collection_permissions WHERE user_id = $1 AND tenant_id = $2 AND table_name = $3",
		colName,
	)
	err = pool.QueryRow(ctx, query, userID, tenantID, tableName).Scan(&allowed)
	if err != nil {
		return false, nil // No permission row = no access
	}
	return allowed, nil
}

// Refresh token support

func SetupRefreshTokensTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS refresh_tokens (
			token TEXT PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func GenerateRefreshToken(ctx context.Context, pool *pgxpool.Pool, userID string) (string, error) {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err := pool.Exec(ctx,
		"INSERT INTO refresh_tokens (token, user_id, expires_at) VALUES ($1, $2, $3)",
		token, userID, expiresAt,
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

func ValidateRefreshToken(ctx context.Context, pool *pgxpool.Pool, token string) (*User, error) {
	var user User
	err := pool.QueryRow(ctx, `
		SELECT u.id, u.email
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token = $1 AND rt.expires_at > NOW()
	`, token).Scan(&user.ID, &user.Email)
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}
	return &user, nil
}

func DeleteRefreshToken(ctx context.Context, pool *pgxpool.Pool, token string) {
	pool.Exec(ctx, "DELETE FROM refresh_tokens WHERE token = $1", token)
}
