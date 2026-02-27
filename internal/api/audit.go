package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RecordAuditLog(pool *pgxpool.Pool, tenantID, email, tableName, action string, payload interface{}) {
	payloadBytes, _ := json.Marshal(payload)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO audit_logs (tenant_id, user_email, table_name, action, payload)
		VALUES ($1, $2, $3, $4, $5)
	`, tenantID, email, tableName, action, payloadBytes)
	if err != nil {
		log.Printf("Failed to record audit log: %v", err)
	}
}

func HandleListAuditLogs(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	tenantID := extractTenantID(r)
	if tenantID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		log.Printf("[Audit] Transaction start error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	// Set tenant scope for RLS
	if _, err := tx.Exec(context.Background(), "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		log.Printf("[Audit] Scope error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	rows, err := tx.Query(context.Background(), `
		SELECT id, user_email, table_name, action, payload, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		log.Printf("[Audit] Query error: %v", err)
		http.Error(w, "Failed to retrieve logs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var id, email, table, action string
		var payload []byte
		var createdAt interface{}
		rows.Scan(&id, &email, &table, &action, &payload, &createdAt)

		var payloadObj interface{}
		json.Unmarshal(payload, &payloadObj)

		logs = append(logs, map[string]interface{}{
			"id":         id,
			"user_email": email,
			"table_name": table,
			"action":     action,
			"payload":    payloadObj,
			"created_at": createdAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if logs == nil {
		logs = []map[string]interface{}{}
	}
	json.NewEncoder(w).Encode(logs)
}
