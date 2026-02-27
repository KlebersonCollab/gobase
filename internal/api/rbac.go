package api

import (
	"context"
	"net/http"

	"gobase/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RBACMiddleware checks collection-level permissions for a specific CRUD action.
// Admin role bypasses all permission checks.
func RBACMiddleware(pool *pgxpool.Pool, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(UserContextKey).(map[string]interface{})
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			role, _ := claims["role"].(string)
			if role == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			userID, _ := claims["sub"].(string)
			tenantID, _ := claims["tenant_id"].(string)
			table := chi.URLParam(r, "table")

			if table == "" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			allowed, err := auth.CheckPermission(context.Background(), pool, userID, tenantID, table, action)
			if err != nil || !allowed {
				http.Error(w, "Forbidden: you don't have "+action+" permission on this collection", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
