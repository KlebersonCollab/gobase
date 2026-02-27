package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const StorageDir = "./data/storage"

func HandleStorageUpload(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	claims, ok := r.Context().Value(UserContextKey).(map[string]interface{})
	if !ok {
		http.Error(w, "Unauthorized Context", http.StatusUnauthorized)
		return
	}
	tenantID, _ := claims["tenant_id"].(string)

	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 1. Prepare Tx
	tx, err := pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Transaction error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Exec(context.Background(), "SELECT set_config('role', 'gobase_api_user', true)"); err != nil {
		http.Error(w, "Failed to apply role", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec(context.Background(), "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		http.Error(w, "Failed to apply tenant scope", http.StatusInternalServerError)
		return
	}

	// 2. Insert Metadata
	var fileID string
	insertQuery := `
		INSERT INTO storage_objects (tenant_id, filename, content_type, size)
		VALUES ($1, $2, $3, $4) RETURNING id
	`
	err = tx.QueryRow(context.Background(), insertQuery, tenantID, handler.Filename, handler.Header.Get("Content-Type"), handler.Size).Scan(&fileID)
	if err != nil {
		log.Printf("[Storage] Upload DB error: %v", err)
		http.Error(w, "Database operation failed", http.StatusInternalServerError)
		return
	}

	// 3. Save File to FS
	tenantDir := filepath.Join(StorageDir, tenantID)
	if err := os.MkdirAll(tenantDir, os.ModePerm); err != nil {
		http.Error(w, "File System Error", http.StatusInternalServerError)
		return
	}

	dstPath := filepath.Join(tenantDir, fileID)
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "File Creation Error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "File Write Error", http.StatusInternalServerError)
		return
	}

	tx.Commit(context.Background())

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"id": fileID})
}

func HandleStorageGet(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	fileID := chi.URLParam(r, "id")

	claims, ok := r.Context().Value(UserContextKey).(map[string]interface{})
	if !ok {
		http.Error(w, "Unauthorized Context", http.StatusUnauthorized)
		return
	}
	tenantID, _ := claims["tenant_id"].(string)

	tx, err := pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Transaction start error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Exec(context.Background(), "SELECT set_config('role', 'gobase_api_user', true)"); err != nil {
		http.Error(w, "Failed to apply role", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec(context.Background(), "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		http.Error(w, "Failed to apply tenant scope", http.StatusInternalServerError)
		return
	}

	var filename, contentType string
	err = tx.QueryRow(context.Background(), "SELECT filename, content_type FROM storage_objects WHERE id = $1", fileID).Scan(&filename, &contentType)
	if err != nil {
		http.Error(w, "File metadata not found or unauthorized", http.StatusNotFound)
		return
	}

	filePath := filepath.Join(StorageDir, tenantID, fileID)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, filePath)
}

func HandleStorageDelete(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	fileID := chi.URLParam(r, "id")

	claims, ok := r.Context().Value(UserContextKey).(map[string]interface{})
	if !ok {
		http.Error(w, "Unauthorized Context", http.StatusUnauthorized)
		return
	}
	tenantID, _ := claims["tenant_id"].(string)

	tx, err := pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Transaction start error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Exec(context.Background(), "SELECT set_config('role', 'gobase_api_user', true)"); err != nil {
		http.Error(w, "Failed to apply role", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec(context.Background(), "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		http.Error(w, "Failed to apply tenant scope", http.StatusInternalServerError)
		return
	}

	res, err := tx.Exec(context.Background(), "DELETE FROM storage_objects WHERE id = $1", fileID)
	if err != nil || res.RowsAffected() == 0 {
		http.Error(w, "File delete unauthorized or missing", http.StatusNotFound)
		return
	}

	tx.Commit(context.Background())

	// Remove from FS
	filePath := filepath.Join(StorageDir, tenantID, fileID)
	os.Remove(filePath) // Ignored on error to keep DB state clean

	w.WriteHeader(http.StatusNoContent)
}
