package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gobase/internal/api"
	"gobase/internal/auth"
	"gobase/internal/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerAndLogin is a test helper that registers a new tenant and returns a JWT.
func registerAndLogin(t *testing.T, ctx context.Context, pool interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (interface{}, error)
}, email, password, tenantName string) string {
	t.Helper()
	// We need the real pool type
	return ""
}

func TestRLSTenantIsolation(t *testing.T) {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		t.Skip("Skipping integration test: DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Clean tables
	pool.Exec(ctx, "DROP TABLE IF EXISTS rls_posts CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS collection_permissions CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS user_tenants CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS refresh_tokens CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS users CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS tenants CASCADE")

	err = auth.SetupAuthTables(ctx, pool)
	require.NoError(t, err)

	router := api.NewRouter(pool)

	// User A (admin of Acme)
	userA, tenantA, err := auth.RegisterNewTenant(ctx, pool, "userA@acme.com", "password", "Acme")
	require.NoError(t, err)
	tokenA, err := auth.GenerateAccessToken(userA, tenantA.TenantID, tenantA.Role)
	require.NoError(t, err)

	// User B (admin of Globex)
	userB, tenantB, err := auth.RegisterNewTenant(ctx, pool, "userB@globex.com", "password", "Globex")
	require.NoError(t, err)
	tokenB, err := auth.GenerateAccessToken(userB, tenantB.TenantID, tenantB.Role)
	require.NoError(t, err)

	// 1. User A creates table (should attach RLS automatically)
	createTablePayload := map[string]interface{}{}
	createTablePayload["name"] = "rls_posts"
	b, _ := json.Marshal(createTablePayload)
	req, _ := http.NewRequest("POST", "/api/schema/tables", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+tokenA)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	// User B also needs the table in their schema context
	// (RLS uses tenant_id, not table ownership, so B can insert if the table exists)
	// But we need to register the table for B's tenant too
	pool.Exec(ctx, "INSERT INTO collection_metadata (table_name, tenant_id) VALUES ('rls_posts', $1) ON CONFLICT DO NOTHING", tenantB.TenantID)

	// Ensure cleanup
	defer pool.Exec(ctx, "DROP TABLE IF EXISTS rls_posts CASCADE")

	// 2. User A inserts a record
	insertA := map[string]interface{}{
		"data": map[string]interface{}{"note": "A secret from A"},
	}
	b, _ = json.Marshal(insertA)
	req, _ = http.NewRequest("POST", "/api/collections/rls_posts", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+tokenA)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code, "Response Body was: %s", rr.Body.String())

	// 3. User B inserts a record
	insertB := map[string]interface{}{
		"data": map[string]interface{}{"note": "A secret from B"},
	}
	b, _ = json.Marshal(insertB)
	req, _ = http.NewRequest("POST", "/api/collections/rls_posts", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+tokenB)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	// 4. User A fetches records (should see EXACTLY 1, which is theirs)
	req, _ = http.NewRequest("GET", "/api/collections/rls_posts", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resultsA []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resultsA)
	assert.Len(t, resultsA, 1)
	assert.Equal(t, "A secret from A", resultsA[0]["data"].(map[string]interface{})["note"])

	// 5. User B fetches records (should see EXACTLY 1, which is theirs)
	req, _ = http.NewRequest("GET", "/api/collections/rls_posts", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resultsB []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resultsB)
	assert.Len(t, resultsB, 1)
	assert.Equal(t, "A secret from B", resultsB[0]["data"].(map[string]interface{})["note"])
}
