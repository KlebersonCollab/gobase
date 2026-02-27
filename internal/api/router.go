package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gobase/internal/auth"
	"gobase/internal/realtime"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouterWithHub(pool *pgxpool.Pool, hub *realtime.Hub) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			// Hardened CSP: No unsafe-eval, strict connect-src, and frame-ancestors
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' fonts.googleapis.com; font-src 'self' data: fonts.gstatic.com; img-src 'self' data:; connect-src 'self' ws: wss: localhost:* 127.0.0.1:* ws://localhost:* ws://127.0.0.1:* wss://localhost:* wss://127.0.0.1:*; frame-ancestors 'none';")
			next.ServeHTTP(w, req)
		})
	})

	r.Get("/", http.RedirectHandler("/admin/", 301).ServeHTTP)
	r.Get("/health", HealthCheck)

	// ── Realtime (WebSocket) ──
	r.Get("/api/realtime", func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			tokenString = r.URL.Query().Get("token")
		}
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")

		tenantID := ""
		if tokenString != "" {
			claims, err := auth.ValidateToken(tokenString)
			if err == nil {
				tenantID, _ = claims["tenant_id"].(string)
			}
		}
		if tenantID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		realtime.ServeWs(hub, tenantID, w, r)
	})

	// ── Auth (Public) ──
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", func(w http.ResponseWriter, r *http.Request) { HandleRegister(w, r, pool) })
		r.Post("/login", func(w http.ResponseWriter, r *http.Request) { HandleLogin(w, r, pool) })
		r.Post("/refresh", func(w http.ResponseWriter, r *http.Request) { HandleRefresh(w, r, pool) })

		// Needs JWT but NOT necessarily tenant-scoped
		r.Group(func(r chi.Router) {
			r.Use(JWTMiddlewareLight(pool))
			r.Post("/select-tenant", func(w http.ResponseWriter, r *http.Request) { HandleSelectTenant(w, r, pool) })
			r.Get("/tenants", func(w http.ResponseWriter, r *http.Request) { HandleListMyTenants(w, r, pool) })
			r.Post("/tenants", func(w http.ResponseWriter, r *http.Request) { HandleCreateTenant(w, r, pool) })
		})

		// Needs full JWT with tenant
		r.Group(func(r chi.Router) {
			r.Use(JWTMiddleware(pool))
			r.Get("/me", func(w http.ResponseWriter, r *http.Request) { HandleMe(w, r, pool) })
			r.Delete("/tenants/{tenantId}", func(w http.ResponseWriter, r *http.Request) { HandleDeleteTenant(w, r, pool) })
		})
	})

	// ── Tenant User & Permission Management (Admin only) ──
	r.Route("/api/users", func(r chi.Router) {
		r.Use(JWTMiddleware(pool))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) { HandleListTenantUsers(w, r, pool) })
		r.Delete("/{userId}", func(w http.ResponseWriter, r *http.Request) { HandleRemoveUserFromTenant(w, r, pool) })
		r.Get("/{userId}/permissions", func(w http.ResponseWriter, r *http.Request) { HandleGetUserPermissions(w, r, pool) })
		r.Put("/{userId}/permissions", func(w http.ResponseWriter, r *http.Request) { HandleUpdateUserPermissions(w, r, pool) })
	})

	// ── Schema Management ──
	r.Route("/api/schema", func(r chi.Router) {
		r.Use(JWTMiddleware(pool))

		// Read-only schema routes (accessible to any authenticated member)
		r.Get("/tables", func(w http.ResponseWriter, req *http.Request) { handleListTables(w, req, pool) })
		r.Get("/tables/{table}/columns", func(w http.ResponseWriter, req *http.Request) { handleGetTableColumns(w, req, pool) })
		r.Get("/tables/{table}/relations", func(w http.ResponseWriter, req *http.Request) { handleGetTableRelations(w, req, pool) })

		// Mutation routes (Admin only)
		adminRouter := r.With(AdminOnly)
		adminRouter.Post("/tables", func(w http.ResponseWriter, req *http.Request) { handleCreateTable(w, req, pool, hub) })
		adminRouter.Delete("/tables/{table}", func(w http.ResponseWriter, req *http.Request) { handleDropTable(w, req, pool, hub) })

		adminRouter.Post("/tables/{table}/columns", func(w http.ResponseWriter, req *http.Request) { handleAddColumn(w, req, pool, hub) })
		adminRouter.Patch("/tables/{table}/columns/{column}", func(w http.ResponseWriter, req *http.Request) { handleUpdateColumn(w, req, pool, hub) })
		adminRouter.Delete("/tables/{table}/columns/{column}", func(w http.ResponseWriter, req *http.Request) { handleDropColumn(w, req, pool, hub) })

		adminRouter.Get("/tables/{table}/columns/{column}/rules", func(w http.ResponseWriter, req *http.Request) { handleGetColumnRules(w, req, pool) })
		adminRouter.Put("/tables/{table}/columns/{column}/rules", func(w http.ResponseWriter, req *http.Request) { handleUpdateColumnRules(w, req, pool, hub) })

		adminRouter.Post("/tables/{table}/relations", func(w http.ResponseWriter, req *http.Request) { handleAddForeignKey(w, req, pool, hub) })
		adminRouter.Delete("/tables/{table}/relations/{constraint}", func(w http.ResponseWriter, req *http.Request) { handleDropForeignKey(w, req, pool, hub) })
	})

	// ── Storage (Protected) ──
	r.Route("/api/storage", func(r chi.Router) {
		r.Use(JWTMiddleware(pool))
		r.Post("/", func(w http.ResponseWriter, r *http.Request) { HandleStorageUpload(w, r, pool) })
		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) { HandleStorageGet(w, r, pool) })
		r.Delete("/{id}", func(w http.ResponseWriter, r *http.Request) { HandleStorageDelete(w, r, pool) })
	})

	// ── Dynamic Collections (Protected + RBAC) ──
	r.Route("/api/collections", func(r chi.Router) {
		r.Use(JWTMiddleware(pool))
		r.Route("/{table}", func(r chi.Router) {
			r.With(RBACMiddleware(pool, "read")).Get("/", func(w http.ResponseWriter, r *http.Request) { HandleDynamicGet(w, r, pool) })
			r.With(RBACMiddleware(pool, "create")).Post("/", func(w http.ResponseWriter, r *http.Request) { HandleDynamicPost(w, r, pool, hub) })
			r.With(RBACMiddleware(pool, "update")).Put("/{id}", func(w http.ResponseWriter, r *http.Request) { HandleDynamicPut(w, r, pool, hub) })
			r.With(RBACMiddleware(pool, "delete")).Delete("/{id}", func(w http.ResponseWriter, r *http.Request) { HandleDynamicDelete(w, r, pool, hub) })
		})
	})

	// ── Audit Logs (Admin Only) ──
	r.Route("/api/audit-logs", func(r chi.Router) {
		r.Use(JWTMiddleware(pool))
		r.Use(AdminOnly)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) { HandleListAuditLogs(w, r, pool) })
	})

	// ── Serve React Admin Frontend ──
	workDir, _ := os.Getwd()
	filesDir := http.Dir(filepath.Join(workDir, "admin-ui", "dist"))
	r.Handle("/admin/*", http.StripPrefix("/admin", http.FileServer(filesDir)))
	r.Get("/admin", http.RedirectHandler("/admin/", 301).ServeHTTP)
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(workDir, "admin-ui", "dist", "favicon.ico"))
	})

	return r
}

func NewRouter(pool *pgxpool.Pool) *chi.Mux {
	hub := realtime.NewHub()
	go hub.Run()
	return NewRouterWithHub(pool, hub)
}
