package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gobase/internal/api"
	"gobase/internal/auth"
	"gobase/internal/database"
	"gobase/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageUploadAndDownload(t *testing.T) {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		t.Skip("Skipping integration test: DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx)
	require.NoError(t, err)
	defer pool.Close()

	pool.Exec(ctx, "DROP TABLE IF EXISTS storage_objects CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS collection_permissions CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS user_tenants CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS refresh_tokens CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS users CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS tenants CASCADE")

	err = auth.SetupAuthTables(ctx, pool)
	require.NoError(t, err)
	err = storage.SetupStorageTables(ctx, pool)
	require.NoError(t, err)

	router := api.NewRouter(pool)

	user, tenant, err := auth.RegisterNewTenant(ctx, pool, "storage_test@acme.com", "password", "AcmeStorage")
	require.NoError(t, err)
	token, err := auth.GenerateAccessToken(user, tenant.TenantID, tenant.Role)
	require.NoError(t, err)

	// Clean out test directory just in case
	os.RemoveAll(api.StorageDir)
	defer os.RemoveAll(api.StorageDir)

	// 1. Upload a File
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", "test_file.txt")
	require.NoError(t, err)
	io.WriteString(fw, "Hello from local storage!")
	w.Close()

	req, _ := http.NewRequest("POST", "/api/storage", &b)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var uploadRes map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &uploadRes)
	require.NoError(t, err)
	fileID := uploadRes["id"].(string)

	// 2. Fetch the File
	reqGet, _ := http.NewRequest("GET", "/api/storage/"+fileID, nil)
	reqGet.Header.Set("Authorization", "Bearer "+token)
	rrGet := httptest.NewRecorder()
	router.ServeHTTP(rrGet, reqGet)

	assert.Equal(t, http.StatusOK, rrGet.Code)
	assert.Equal(t, "Hello from local storage!", rrGet.Body.String())
	assert.Equal(t, `attachment; filename="test_file.txt"`, rrGet.Header().Get("Content-Disposition"))

	// 3. Delete the file
	reqDel, _ := http.NewRequest("DELETE", "/api/storage/"+fileID, nil)
	reqDel.Header.Set("Authorization", "Bearer "+token)
	rrDel := httptest.NewRecorder()
	router.ServeHTTP(rrDel, reqDel)

	assert.Equal(t, http.StatusNoContent, rrDel.Code)

	// Ensures OS File was destroyed
	files, _ := filepath.Glob(filepath.Join(api.StorageDir, "*", fileID))
	assert.Empty(t, files)
}
