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

func TestAuthEndpointsAndMiddleware(t *testing.T) {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		t.Skip("Skipping integration test: DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Ensure DB is clean
	pool.Exec(ctx, "DROP TABLE IF EXISTS users CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS tenants CASCADE")

	err = auth.SetupAuthTables(ctx, pool)
	require.NoError(t, err)

	router := api.NewRouter(pool)

	// 1. Test Register Endpoint
	registerPayload := map[string]interface{}{
		"email":       "api_test@gobase.com",
		"password":    "secure_password",
		"tenant_name": "API Corp",
	}
	body, _ := json.Marshal(registerPayload)

	req, _ := http.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	// 2. Test Login Endpoint
	loginPayload := map[string]interface{}{
		"email":    "api_test@gobase.com",
		"password": "secure_password",
	}
	body, _ = json.Marshal(loginPayload)

	req, _ = http.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var loginResponse map[string]string
	err = json.NewDecoder(rr.Body).Decode(&loginResponse)
	assert.NoError(t, err)
	token := loginResponse["token"]
	assert.NotEmpty(t, token)

	// 3. Test JWT Middleware blocking unauthorized
	req, _ = http.NewRequest("GET", "/api/collections/any_table", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	// 4. Test JWT Middleware allowing authorized (will hit DB and likely fail on table missing, but passes 401)
	req, _ = http.NewRequest("GET", "/api/collections/any_table", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusUnauthorized, rr.Code)
}
