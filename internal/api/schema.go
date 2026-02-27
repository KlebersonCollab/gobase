package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"gobase/internal/realtime"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrateCollectionMetadata creates the metadata table that tracks tenant ownership of collections.
func MigrateCollectionMetadata(ctx context.Context, pool *pgxpool.Pool) error {
	// Existing table ownership metadata
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS collection_metadata (
			table_name TEXT PRIMARY KEY,
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	// New column-level metadata (Validation Rules)
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS column_metadata (
			table_name TEXT NOT NULL,
			column_name TEXT NOT NULL,
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			rules JSONB DEFAULT '{}' NOT NULL,
			PRIMARY KEY (table_name, column_name, tenant_id)
		);
	`)
	if err != nil {
		return err
	}

	// Ensure the realtime broadcast function exists
	if err := realtime.EnsureBroadcastFunction(pool); err != nil {
		return err
	}

	// Apply triggers to all tracked tables (self-healing)
	rows, err := pool.Query(ctx, "SELECT table_name FROM collection_metadata")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tName string
			if err := rows.Scan(&tName); err == nil {
				realtime.ApplyRealtimeTrigger(pool, tName)
			}
		}
	}

	return nil
}

// extractTenantID reads the tenant_id from the JWT claims in the request context.
func extractTenantID(r *http.Request) string {
	claims, ok := r.Context().Value(UserContextKey).(map[string]interface{})
	if !ok {
		return ""
	}
	tid, _ := claims["tenant_id"].(string)
	return tid
}

func extractUserEmail(r *http.Request) string {
	claims, ok := r.Context().Value(UserContextKey).(map[string]interface{})
	if !ok {
		return ""
	}
	email, _ := claims["email"].(string)
	return email
}

type CreateTableRequest struct {
	Name string `json:"name"`
}

func handleListTables(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	tenantID := extractTenantID(r)
	if tenantID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := `SELECT table_name FROM collection_metadata WHERE tenant_id = $1 ORDER BY table_name`
	rows, err := pool.Query(context.Background(), query, tenantID)
	if err != nil {
		http.Error(w, "Failed to list tables", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		tables = append(tables, name)
	}
	if tables == nil {
		tables = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

func handleCreateTable(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	tenantID := extractTenantID(r)
	if tenantID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid syntax", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Table name is required", http.StatusBadRequest)
		return
	}

	// Dynamic DDL execution
	sName := pgx.Identifier{req.Name}.Sanitize()
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
			modified_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
			created_by TEXT DEFAULT '' NOT NULL,
			modified_by TEXT DEFAULT '' NOT NULL,
			tenant_id UUID NOT NULL
		);
		GRANT ALL ON %s TO gobase_api_user;
		ALTER TABLE %s ENABLE ROW LEVEL SECURITY;
		ALTER TABLE %s FORCE ROW LEVEL SECURITY;
		DROP POLICY IF EXISTS tenant_isolation_policy ON %s;
		CREATE POLICY tenant_isolation_policy ON %s
			USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
	`, sName, sName, sName, sName, sName, sName)

	_, err := pool.Exec(context.Background(), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create table: %v", err), http.StatusInternalServerError)
		return
	}

	// Register ownership in collection_metadata
	_, err = pool.Exec(context.Background(),
		`INSERT INTO collection_metadata (table_name, tenant_id) VALUES ($1, $2) ON CONFLICT (table_name) DO NOTHING`,
		req.Name, tenantID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Table created but failed to register ownership: %v", err), http.StatusInternalServerError)
		return
	}

	// Apply Realtime triggers so the Hub gets notified of changes
	realtime.ApplyRealtimeTrigger(pool, req.Name)

	if hub != nil {
		hub.Broadcast <- &realtime.BroadcastMessage{
			TenantID: tenantID,
			Table:    "schema",
			Action:   "SCHEMA_CREATE_TABLE",
			Data:     map[string]interface{}{"table": req.Name},
		}
	}

	RecordAuditLog(pool, tenantID, extractUserEmail(r), req.Name, "SCHEMA_CREATE_TABLE", req)

	w.WriteHeader(http.StatusCreated)
}

func handleDropTable(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	tenantID := extractTenantID(r)
	tableName := chi.URLParam(r, "table")
	if tenantID == "" || tableName == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Verify tenant owns this table
	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM collection_metadata WHERE table_name = $1 AND tenant_id = $2`,
		tableName, tenantID).Scan(&count)
	if err != nil || count == 0 {
		http.Error(w, "Collection not found or not owned by your tenant", http.StatusForbidden)
		return
	}

	// Drop table and metadata
	sName := pgx.Identifier{tableName}.Sanitize()
	_, err = pool.Exec(context.Background(), fmt.Sprintf(`DROP TABLE IF EXISTS %s CASCADE`, sName))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to drop table: %v", err), http.StatusInternalServerError)
		return
	}
	pool.Exec(context.Background(), `DELETE FROM collection_metadata WHERE table_name = $1`, tableName)

	if hub != nil {
		hub.Broadcast <- &realtime.BroadcastMessage{
			TenantID: tenantID,
			Table:    "schema",
			Action:   "SCHEMA_DROP_TABLE",
			Data:     map[string]interface{}{"table": tableName},
		}
	}

	RecordAuditLog(pool, tenantID, extractUserEmail(r), tableName, "SCHEMA_DROP_TABLE", map[string]string{"table": tableName})

	w.WriteHeader(http.StatusNoContent)
}

type AddColumnRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type UpdateColumnRequest struct {
	NewName string `json:"new_name"`
	NewType string `json:"new_type"`
}

func handleAddColumn(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	tableName := chi.URLParam(r, "table")
	var req AddColumnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid syntax", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Type == "" {
		http.Error(w, "Column name and type are required", http.StatusBadRequest)
		return
	}

	// Safe DB Data types mapping
	allowedTypes := map[string]string{
		"text":      "TEXT",
		"uuid":      "UUID",
		"timestamp": "TIMESTAMP WITH TIME ZONE",
		"integer":   "INTEGER",
		"boolean":   "BOOLEAN",
		"jsonb":     "JSONB",
	}

	pgType, ok := allowedTypes[req.Type]
	if !ok {
		http.Error(w, "Unsupported column type", http.StatusBadRequest)
		return
	}

	sTable := pgx.Identifier{tableName}.Sanitize()
	sCol := pgx.Identifier{req.Name}.Sanitize()
	query := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s`, sTable, sCol, pgType)
	if req.Required {
		query += " NOT NULL"
	}

	_, err := pool.Exec(context.Background(), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to add column: %v", err), http.StatusInternalServerError)
		return
	}

	if hub != nil {
		hub.Broadcast <- &realtime.BroadcastMessage{
			TenantID: extractTenantID(r),
			Table:    tableName,
			Action:   "SCHEMA_ADD_COLUMN",
			Data:     map[string]interface{}{"column": req.Name, "type": req.Type},
		}
	}

	RecordAuditLog(pool, extractTenantID(r), extractUserEmail(r), tableName, "SCHEMA_ADD_COLUMN", req)

	w.WriteHeader(http.StatusOK)
}

func handleGetColumnRules(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	tableName := chi.URLParam(r, "table")
	colName := chi.URLParam(r, "column")
	tenantID := extractTenantID(r)

	var rules []byte
	err := pool.QueryRow(context.Background(), `
		SELECT rules FROM column_metadata 
		WHERE table_name = $1 AND column_name = $2 AND tenant_id = $3
	`, tableName, colName, tenantID).Scan(&rules)

	if err != nil {
		// If not found, return empty object
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(rules)
}

func handleUpdateColumnRules(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	tableName := chi.URLParam(r, "table")
	colName := chi.URLParam(r, "column")
	tenantID := extractTenantID(r)

	var rules map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
		http.Error(w, "Invalid JSON rules", http.StatusBadRequest)
		return
	}

	rulesJSON, _ := json.Marshal(rules)

	_, err := pool.Exec(context.Background(), `
		INSERT INTO column_metadata (table_name, column_name, tenant_id, rules)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (table_name, column_name, tenant_id) 
		DO UPDATE SET rules = EXCLUDED.rules
	`, tableName, colName, tenantID, rulesJSON)

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save rules: %v", err), http.StatusInternalServerError)
		return
	}

	if hub != nil {
		hub.Broadcast <- &realtime.BroadcastMessage{
			TenantID: tenantID,
			Table:    tableName,
			Action:   "SCHEMA_UPDATE_RULES",
			Data:     map[string]interface{}{"column": colName, "rules": rules},
		}
	}

	w.WriteHeader(http.StatusOK)
}

func handleGetTableColumns(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	tableName := chi.URLParam(r, "table")

	query := `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_name = $1 AND table_schema = 'public'
		ORDER BY ordinal_position;
	`

	rows, err := pool.Query(context.Background(), query, tableName)
	if err != nil {
		http.Error(w, "Failed to inspect table schema", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var columns []map[string]interface{}
	for rows.Next() {
		var colName, dataType, isNullable string
		if err := rows.Scan(&colName, &dataType, &isNullable); err != nil {
			continue
		}
		columns = append(columns, map[string]interface{}{
			"name":     colName,
			"type":     dataType,
			"required": isNullable == "NO",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(columns)
}

func handleDropColumn(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	tableName := chi.URLParam(r, "table")
	colName := chi.URLParam(r, "column")

	if colName == "id" || colName == "tenant_id" || colName == "created_at" {
		http.Error(w, "Cannot drop core system columns", http.StatusBadRequest)
		return
	}

	sTable := pgx.Identifier{tableName}.Sanitize()
	sCol := pgx.Identifier{colName}.Sanitize()
	query := fmt.Sprintf(`ALTER TABLE %s DROP COLUMN IF EXISTS %s`, sTable, sCol)

	_, err := pool.Exec(context.Background(), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to drop column: %v", err), http.StatusInternalServerError)
		return
	}

	if hub != nil {
		hub.Broadcast <- &realtime.BroadcastMessage{
			TenantID: extractTenantID(r),
			Table:    tableName,
			Action:   "SCHEMA_DROP_COLUMN",
			Data:     map[string]interface{}{"column": colName},
		}
	}

	RecordAuditLog(pool, extractTenantID(r), extractUserEmail(r), tableName, "SCHEMA_DROP_COLUMN", map[string]string{"column": colName})

	w.WriteHeader(http.StatusOK)
}

func handleUpdateColumn(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	tableName := chi.URLParam(r, "table")
	oldName := chi.URLParam(r, "column")
	var req UpdateColumnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid syntax", http.StatusBadRequest)
		return
	}

	if oldName == "id" || oldName == "tenant_id" || oldName == "created_at" {
		http.Error(w, "Cannot edit core system columns", http.StatusBadRequest)
		return
	}

	tenantID := extractTenantID(r)
	userEmail := extractUserEmail(r)

	// Rename column if requested
	if req.NewName != "" && req.NewName != oldName {
		sTable := pgx.Identifier{tableName}.Sanitize()
		sOld := pgx.Identifier{oldName}.Sanitize()
		sNew := pgx.Identifier{req.NewName}.Sanitize()
		query := fmt.Sprintf(`ALTER TABLE %s RENAME COLUMN %s TO %s`, sTable, sOld, sNew)
		if _, err := pool.Exec(context.Background(), query); err != nil {
			http.Error(w, "Failed to rename column", http.StatusInternalServerError)
			return
		}
		// Also update column_metadata
		pool.Exec(context.Background(), `
			UPDATE column_metadata SET column_name = $1 
			WHERE table_name = $2 AND column_name = $3 AND tenant_id = $4
		`, req.NewName, tableName, oldName, tenantID)

		oldName = req.NewName // For subsequent type change or logging
	}

	// Change type if requested
	if req.NewType != "" {
		allowedTypes := map[string]string{
			"text":      "TEXT",
			"uuid":      "UUID",
			"timestamp": "TIMESTAMP WITH TIME ZONE",
			"integer":   "INTEGER",
			"boolean":   "BOOLEAN",
			"jsonb":     "JSONB",
		}
		pgType, ok := allowedTypes[req.NewType]
		if !ok {
			http.Error(w, "Unsupported column type", http.StatusBadRequest)
			return
		}

		sTable := pgx.Identifier{tableName}.Sanitize()
		sCol := pgx.Identifier{oldName}.Sanitize()
		// Use intermediate text cast for better compatibility (e.g., jsonb -> text -> uuid)
		query := fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::text::%s`, sTable, sCol, pgType, sCol, pgType)
		if _, err := pool.Exec(context.Background(), query); err != nil {
			log.Printf("[Schema] Failed to change column type: %v", err)
			http.Error(w, fmt.Sprintf("Failed to change column type: %v", err), http.StatusInternalServerError)
			return
		}
	}

	if hub != nil {
		hub.Broadcast <- &realtime.BroadcastMessage{
			TenantID: tenantID,
			Table:    tableName,
			Action:   "SCHEMA_UPDATE_COLUMN",
			Data:     map[string]interface{}{"column": oldName, "new_name": req.NewName, "new_type": req.NewType},
		}
	}

	RecordAuditLog(pool, tenantID, userEmail, tableName, "SCHEMA_UPDATE_COLUMN", req)

	w.WriteHeader(http.StatusOK)
}

// --- Foreign Key Management ---

type AddForeignKeyRequest struct {
	Column           string `json:"column"`
	ReferencesTable  string `json:"references_table"`
	ReferencesColumn string `json:"references_column"`
	OnDelete         string `json:"on_delete"`
}

func handleAddForeignKey(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	tableName := chi.URLParam(r, "table")
	var req AddForeignKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid syntax", http.StatusBadRequest)
		return
	}

	if req.Column == "" || req.ReferencesTable == "" {
		http.Error(w, "column and references_table are required", http.StatusBadRequest)
		return
	}

	if req.ReferencesColumn == "" {
		req.ReferencesColumn = "id"
	}

	allowedActions := map[string]bool{
		"CASCADE": true, "SET NULL": true, "RESTRICT": true, "NO ACTION": true,
	}
	if req.OnDelete == "" {
		req.OnDelete = "CASCADE"
	}
	if !allowedActions[req.OnDelete] {
		http.Error(w, "Invalid on_delete action. Use: CASCADE, SET NULL, RESTRICT, NO ACTION", http.StatusBadRequest)
		return
	}

	constraintName := fmt.Sprintf("fk_%s_%s", tableName, req.Column)
	sTable := pgx.Identifier{tableName}.Sanitize()
	sConstraint := pgx.Identifier{constraintName}.Sanitize()
	sCol := pgx.Identifier{req.Column}.Sanitize()
	sRefTable := pgx.Identifier{req.ReferencesTable}.Sanitize()
	sRefCol := pgx.Identifier{req.ReferencesColumn}.Sanitize()

	query := fmt.Sprintf(
		`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s) ON DELETE %s`,
		sTable, sConstraint, sCol, sRefTable, sRefCol, req.OnDelete,
	)

	_, err := pool.Exec(context.Background(), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to add foreign key: %v", err), http.StatusInternalServerError)
		return
	}

	if hub != nil {
		hub.Broadcast <- &realtime.BroadcastMessage{
			TenantID: extractTenantID(r),
			Table:    tableName,
			Action:   "SCHEMA_ADD_FK",
			Data:     map[string]interface{}{"column": req.Column, "references": req.ReferencesTable},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"constraint": constraintName,
		"message":    fmt.Sprintf("FK %s.%s → %s.%s created", tableName, req.Column, req.ReferencesTable, req.ReferencesColumn),
	})
}

func handleGetTableRelations(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	tableName := chi.URLParam(r, "table")

	query := `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS references_table,
			ccu.column_name AS references_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_name = $1
			AND tc.table_schema = 'public'
		ORDER BY tc.constraint_name;
	`

	rows, err := pool.Query(context.Background(), query, tableName)
	if err != nil {
		http.Error(w, "Failed to inspect relations", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var relations []map[string]string
	for rows.Next() {
		var constraintName, column, refTable, refColumn string
		if err := rows.Scan(&constraintName, &column, &refTable, &refColumn); err != nil {
			continue
		}
		relations = append(relations, map[string]string{
			"constraint_name":   constraintName,
			"column":            column,
			"references_table":  refTable,
			"references_column": refColumn,
		})
	}

	if relations == nil {
		relations = []map[string]string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(relations)
}

func handleDropForeignKey(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	tableName := chi.URLParam(r, "table")
	constraintName := chi.URLParam(r, "constraint")

	sTable := pgx.Identifier{tableName}.Sanitize()
	sConstraint := pgx.Identifier{constraintName}.Sanitize()

	query := fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s`, sTable, sConstraint)

	if _, err := pool.Exec(context.Background(), query); err != nil {
		http.Error(w, fmt.Sprintf("Failed to drop constraint: %v", err), http.StatusInternalServerError)
		return
	}

	if hub != nil {
		hub.Broadcast <- &realtime.BroadcastMessage{
			TenantID: extractTenantID(r),
			Table:    tableName,
			Action:   "SCHEMA_DROP_FK",
			Data:     map[string]interface{}{"constraint": constraintName},
		}
	}

	w.WriteHeader(http.StatusOK)
}
