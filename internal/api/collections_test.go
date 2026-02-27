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

func TestDynamicCollectionsCRUD(t *testing.T) {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		t.Skip("Skipping integration test: DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Ensure cleanup and create test table for CRUD
	pool.Exec(ctx, "DROP TABLE IF EXISTS test_crud")
	_, err = pool.Exec(ctx, `
		CREATE TABLE test_crud (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			tenant_id UUID,
			title TEXT,
			data JSONB DEFAULT '{}'::jsonb
		);
		GRANT ALL ON test_crud TO gobase_api_user;
	`)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DROP TABLE IF EXISTS test_crud")

	router := api.NewRouter(pool)

	// 1. Test POST (Insert Record)
	insertPayload := map[string]interface{}{
		"title": "My first dynamic post",
		"data": map[string]interface{}{
			"tags":     []string{"go", "api"},
			"views":    100,
			"is_draft": false,
		},
	}
	body, _ := json.Marshal(insertPayload)

	req, _ := http.NewRequest("POST", "/api/collections/test_crud", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+createTestToken())

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "Response Body was: %s", rr.Body.String())

	// Validate the response contains the new ID
	var insertResponse map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&insertResponse)
	assert.NoError(t, err)
	recordId, ok := insertResponse["id"].(string)
	assert.True(t, ok, "Insert response should contain the generated ID")
	assert.NotEmpty(t, recordId)

	// 2. Test GET (Fetch the created record)
	req, _ = http.NewRequest("GET", "/api/collections/test_crud?id=eq."+recordId, nil)
	req.Header.Set("Authorization", "Bearer "+createTestToken())
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var getResponse []map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&getResponse)
	assert.NoError(t, err)
	assert.Len(t, getResponse, 1)
	assert.Equal(t, "My first dynamic post", getResponse[0]["title"])

	// Data nested check
	dataObj := getResponse[0]["data"].(map[string]interface{})
	assert.Equal(t, float64(100), dataObj["views"]) // Json unmarshals numbers to float64

	// 3. Test PUT (Update Record)
	updatePayload := map[string]interface{}{
		"title": "Updated Title",
		"data": map[string]interface{}{
			"tags": []string{"updated"},
		},
	}
	body, _ = json.Marshal(updatePayload)
	req, _ = http.NewRequest("PUT", "/api/collections/test_crud/"+recordId, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+createTestToken())

	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Validate Update
	var finalTitle string
	err = pool.QueryRow(ctx, "SELECT title FROM test_crud WHERE id = $1", recordId).Scan(&finalTitle)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", finalTitle)

	// 4. Test DELETE
	req, _ = http.NewRequest("DELETE", "/api/collections/test_crud/"+recordId, nil)
	req.Header.Set("Authorization", "Bearer "+createTestToken())
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Validate Delete
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM test_crud WHERE id = $1", recordId).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
