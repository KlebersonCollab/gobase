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
	"gobase/internal/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDynamicSchemaCreation(t *testing.T) {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		t.Skip("Skipping integration test: DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Ensure cleanup
	defer func() {
		pool.Exec(ctx, "DROP TABLE IF EXISTS test_dynamic_table")
	}()

	router := api.NewRouter(pool)

	// 1. Test Create Table
	createPayload := map[string]interface{}{
		"name": "test_dynamic_table",
	}
	body, _ := json.Marshal(createPayload)

	req, _ := http.NewRequest("POST", "/api/schema/tables", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+createTestToken())

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	// Verify table was actually created in DB
	var tableName string
	err = pool.QueryRow(ctx, "SELECT table_name FROM information_schema.tables WHERE table_name = 'test_dynamic_table'").Scan(&tableName)
	assert.NoError(t, err)
	assert.Equal(t, "test_dynamic_table", tableName)

	// 2. Test Add Column
	addColumnPayload := map[string]interface{}{
		"name": "title",
		"type": "text",
	}
	body, _ = json.Marshal(addColumnPayload)

	req, _ = http.NewRequest("POST", "/api/schema/tables/test_dynamic_table/columns", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+createTestToken())

	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify column exists
	var columnName string
	err = pool.QueryRow(ctx, "SELECT column_name FROM information_schema.columns WHERE table_name = 'test_dynamic_table' AND column_name = 'title'").Scan(&columnName)
	assert.NoError(t, err)
	assert.Equal(t, "title", columnName)
}
