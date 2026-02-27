package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gobase/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ── Register ──

type RegisterRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantName string `json:"tenant_name"` // required for both modes
	Mode       string `json:"mode"`        // "create" or "join"
}

func HandleRegister(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Payload", http.StatusBadRequest)
		return
	}

	if req.Email == "" || len(req.Password) < 6 || req.TenantName == "" {
		http.Error(w, "Need email, password (min 6 chars), and tenant_name", http.StatusBadRequest)
		return
	}

	mode := req.Mode
	if mode == "" {
		mode = "create"
	}

	var user *auth.User
	var tenant *auth.UserTenant
	var err error

	switch mode {
	case "create":
		user, tenant, err = auth.RegisterNewTenant(context.Background(), pool, req.Email, req.Password, req.TenantName)
	case "join":
		user, tenant, err = auth.RegisterJoinTenant(context.Background(), pool, req.Email, req.Password, req.TenantName)
	default:
		http.Error(w, "mode must be 'create' or 'join'", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("[Auth] Registration error: %v", err)
		http.Error(w, "Registration failed", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":   user,
		"tenant": tenant,
	})
}

// ── Login ──

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func HandleLogin(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Payload", http.StatusBadRequest)
		return
	}

	user, tenants, err := auth.Login(context.Background(), pool, req.Email, req.Password)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var token, refreshToken string

	if len(tenants) == 1 {
		// Single tenant → full JWT
		token, _ = auth.GenerateAccessToken(user, tenants[0].TenantID, tenants[0].Role)
	} else {
		// Multi-tenant → light JWT (no tenant, used for select-tenant call)
		token, _ = auth.GenerateAccessToken(user, "", "")
	}
	refreshToken, _ = auth.GenerateRefreshToken(context.Background(), pool, user.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":          user,
		"tenants":       tenants,
		"token":         token,
		"refresh_token": refreshToken,
	})
}

// ── Select Tenant ──

type SelectTenantRequest struct {
	TenantID string `json:"tenant_id"`
}

func HandleSelectTenant(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	claims, ok := r.Context().Value(UserContextKey).(map[string]interface{})
	if !ok {
		// User might not have a full JWT yet (just logged in), try from a minimal auth
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)

	var req SelectTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TenantID == "" {
		http.Error(w, "Missing tenant_id", http.StatusBadRequest)
		return
	}

	// Verify user belongs to this tenant
	var role string
	err := pool.QueryRow(context.Background(),
		"SELECT role FROM user_tenants WHERE user_id = $1 AND tenant_id = $2",
		userID, req.TenantID,
	).Scan(&role)
	if err != nil {
		http.Error(w, "You don't belong to this tenant", http.StatusForbidden)
		return
	}

	user := &auth.User{ID: userID, Email: email}
	token, _ := auth.GenerateAccessToken(user, req.TenantID, role)
	refreshToken, _ := auth.GenerateRefreshToken(context.Background(), pool, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":         token,
		"refresh_token": refreshToken,
	})
}

// ── Me ──

func HandleMe(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	claims, ok := r.Context().Value(UserContextKey).(map[string]interface{})
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        claims["sub"],
		"email":     claims["email"],
		"tenant_id": claims["tenant_id"],
		"role":      claims["role"],
	})
}

// ── Refresh ──

func HandleRefresh(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
		TenantID     string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		http.Error(w, "Missing refresh_token", http.StatusBadRequest)
		return
	}

	user, err := auth.ValidateRefreshToken(context.Background(), pool, req.RefreshToken)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	auth.DeleteRefreshToken(context.Background(), pool, req.RefreshToken)

	// If tenant_id provided, use it. Otherwise get first tenant
	tenantID := req.TenantID
	role := "member"
	if tenantID == "" {
		tenants, _ := auth.GetUserTenants(context.Background(), pool, user.ID)
		if len(tenants) > 0 {
			tenantID = tenants[0].TenantID
			role = tenants[0].Role
		}
	} else {
		pool.QueryRow(context.Background(),
			"SELECT role FROM user_tenants WHERE user_id = $1 AND tenant_id = $2",
			user.ID, tenantID,
		).Scan(&role)
	}

	newAccessToken, _ := auth.GenerateAccessToken(user, tenantID, role)
	newRefreshToken, _ := auth.GenerateRefreshToken(context.Background(), pool, user.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":         newAccessToken,
		"refresh_token": newRefreshToken,
	})
}

// ── Tenant Management (Admin) ──

func HandleListMyTenants(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	claims, _ := r.Context().Value(UserContextKey).(map[string]interface{})
	userID, _ := claims["sub"].(string)

	tenants, err := auth.GetUserTenants(context.Background(), pool, userID)
	if err != nil {
		http.Error(w, "Failed to list tenants", http.StatusInternalServerError)
		return
	}
	if tenants == nil {
		tenants = []auth.UserTenant{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tenants)
}

func HandleCreateTenant(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	claims, _ := r.Context().Value(UserContextKey).(map[string]interface{})
	userID, _ := claims["sub"].(string)

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "Missing name", http.StatusBadRequest)
		return
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	var tenantID string
	err = tx.QueryRow(context.Background(), "INSERT INTO tenants (name) VALUES ($1) RETURNING id", req.Name).Scan(&tenantID)
	if err != nil {
		http.Error(w, "Tenant name already exists", http.StatusConflict)
		return
	}

	_, err = tx.Exec(context.Background(),
		"INSERT INTO user_tenants (user_id, tenant_id, role) VALUES ($1, $2, 'admin')",
		userID, tenantID,
	)
	if err != nil {
		http.Error(w, "Error linking tenant", http.StatusInternalServerError)
		return
	}

	tx.Commit(context.Background())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": tenantID, "name": req.Name})
}

func HandleDeleteTenant(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	claims, _ := r.Context().Value(UserContextKey).(map[string]interface{})
	userID, _ := claims["sub"].(string)
	tenantID := chi.URLParam(r, "tenantId")

	// Must be admin of the tenant to delete it
	var role string
	err := pool.QueryRow(context.Background(),
		"SELECT role FROM user_tenants WHERE user_id = $1 AND tenant_id = $2",
		userID, tenantID,
	).Scan(&role)
	if err != nil || role != "admin" {
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}

	// Get all collection names for this tenant to drop them
	rows, err := pool.Query(context.Background(),
		"SELECT table_name FROM collection_metadata WHERE tenant_id = $1", tenantID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tableName string
			rows.Scan(&tableName)
			pool.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", pgx.Identifier{tableName}.Sanitize()))
		}
	}

	// Delete from collection_metadata
	pool.Exec(context.Background(), "DELETE FROM collection_metadata WHERE tenant_id = $1", tenantID)

	// Delete tenant (cascades user_tenants, collection_permissions, column_metadata)
	pool.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", tenantID)

	w.WriteHeader(http.StatusNoContent)
}

// ── Tenant Users Management (Admin) ──

func HandleListTenantUsers(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	claims, _ := r.Context().Value(UserContextKey).(map[string]interface{})
	tenantID, _ := claims["tenant_id"].(string)
	role, _ := claims["role"].(string)

	if role != "admin" {
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}

	rows, err := pool.Query(context.Background(), `
		SELECT u.id, u.email, ut.role, ut.created_at
		FROM user_tenants ut
		JOIN users u ON u.id = ut.user_id
		WHERE ut.tenant_id = $1
		ORDER BY ut.created_at
	`, tenantID)
	if err != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type TenantUser struct {
		ID        string    `json:"id"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		CreatedAt time.Time `json:"created_at"`
	}

	var users []TenantUser
	for rows.Next() {
		var u TenantUser
		rows.Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt)
		users = append(users, u)
	}
	if users == nil {
		users = []TenantUser{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func HandleRemoveUserFromTenant(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	claims, _ := r.Context().Value(UserContextKey).(map[string]interface{})
	tenantID, _ := claims["tenant_id"].(string)
	role, _ := claims["role"].(string)
	targetUserID := chi.URLParam(r, "userId")

	if role != "admin" {
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}

	pool.Exec(context.Background(), "DELETE FROM user_tenants WHERE user_id = $1 AND tenant_id = $2", targetUserID, tenantID)
	pool.Exec(context.Background(), "DELETE FROM collection_permissions WHERE user_id = $1 AND tenant_id = $2", targetUserID, tenantID)

	w.WriteHeader(http.StatusNoContent)
}

// ── Permissions Management ──

func HandleGetUserPermissions(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	claims, _ := r.Context().Value(UserContextKey).(map[string]interface{})
	tenantID, _ := claims["tenant_id"].(string)
	role, _ := claims["role"].(string)
	targetUserID := chi.URLParam(r, "userId")

	if role != "admin" {
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}

	rows, err := pool.Query(context.Background(), `
		SELECT table_name, can_read, can_create, can_update, can_delete
		FROM collection_permissions
		WHERE user_id = $1 AND tenant_id = $2
	`, targetUserID, tenantID)
	if err != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Perm struct {
		TableName string `json:"table_name"`
		CanRead   bool   `json:"can_read"`
		CanCreate bool   `json:"can_create"`
		CanUpdate bool   `json:"can_update"`
		CanDelete bool   `json:"can_delete"`
	}

	var perms []Perm
	for rows.Next() {
		var p Perm
		rows.Scan(&p.TableName, &p.CanRead, &p.CanCreate, &p.CanUpdate, &p.CanDelete)
		perms = append(perms, p)
	}
	if perms == nil {
		perms = []Perm{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perms)
}

func HandleUpdateUserPermissions(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	claims, _ := r.Context().Value(UserContextKey).(map[string]interface{})
	tenantID, _ := claims["tenant_id"].(string)
	role, _ := claims["role"].(string)
	targetUserID := chi.URLParam(r, "userId")

	if role != "admin" {
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}

	type PermUpdate struct {
		TableName string `json:"table_name"`
		CanRead   bool   `json:"can_read"`
		CanCreate bool   `json:"can_create"`
		CanUpdate bool   `json:"can_update"`
		CanDelete bool   `json:"can_delete"`
	}

	var perms []PermUpdate
	if err := json.NewDecoder(r.Body).Decode(&perms); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	// Delete existing permissions for this user in this tenant
	tx.Exec(context.Background(),
		"DELETE FROM collection_permissions WHERE user_id = $1 AND tenant_id = $2",
		targetUserID, tenantID,
	)

	// Insert new permissions
	for _, p := range perms {
		tx.Exec(context.Background(), `
			INSERT INTO collection_permissions (user_id, tenant_id, table_name, can_read, can_create, can_update, can_delete)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, targetUserID, tenantID, p.TableName, p.CanRead, p.CanCreate, p.CanUpdate, p.CanDelete)
	}

	tx.Commit(context.Background())

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ── Middleware ──

type ContextKey string

const UserContextKey = ContextKey("userContext")

func JWTMiddleware(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := auth.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			userID, _ := claims["sub"].(string)
			if userID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			var exists bool
			pool.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
			if !exists {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, map[string]interface{}{
				"sub":       claims["sub"],
				"email":     claims["email"],
				"tenant_id": claims["tenant_id"],
				"role":      claims["role"],
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// JWTMiddlewareLight validates JWT but does NOT require tenant_id (for pre-tenant-selection calls)
func JWTMiddlewareLight(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := auth.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserContextKey, map[string]interface{}{
				"sub":       claims["sub"],
				"email":     claims["email"],
				"tenant_id": claims["tenant_id"],
				"role":      claims["role"],
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminOnly middleware checks that the user has the 'admin' role.
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(UserContextKey).(map[string]interface{})
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		role, _ := claims["role"].(string)
		if role != "admin" {
			http.Error(w, "Forbidden: admin only", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
