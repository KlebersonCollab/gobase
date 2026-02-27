package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"gobase/internal/realtime"
	"gobase/internal/validator"

	"github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var psql = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

func HandleDynamicGet(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	table := chi.URLParam(r, "table")

	claims, ok := r.Context().Value(UserContextKey).(map[string]interface{})
	if !ok {
		http.Error(w, "Unauthorized Context", http.StatusUnauthorized)
		return
	}
	tenantID, _ := claims["tenant_id"].(string)

	// Base query Builder
	sTable := pgx.Identifier{table}.Sanitize()
	q := psql.Select("*").From(sTable)

	// Apply PostgREST-style filters, ordering, pagination
	q = ApplyFilters(q, r.URL.Query(), table)

	sql, args, err := q.ToSql()
	if err != nil {
		http.Error(w, "Query builder error", http.StatusInternalServerError)
		return
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Transaction start error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	// Switch role to a non-superuser so RLS triggers
	if _, err := tx.Exec(context.Background(), "SELECT set_config('role', 'gobase_api_user', true)"); err != nil {
		http.Error(w, "Failed to apply role", http.StatusInternalServerError)
		return
	}
	// Inject Tenant ID scoped to this transaction
	if _, err := tx.Exec(context.Background(), "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		http.Error(w, "Failed to apply tenant scope", http.StatusInternalServerError)
		return
	}

	rows, err := tx.Query(context.Background(), sql, args...)
	if err != nil {
		log.Printf("[Collections] Query error: %v (SQL: %s)", err, sql)
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		results = append(results, scanRowToMap(rows))
	}

	w.Header().Set("Content-Type", "application/json")
	if results == nil {
		results = []map[string]interface{}{}
	}

	// Resolve embedded relations: ?select=*,tasks(*)
	embeddedRelations := ParseEmbeddedRelations(r.URL.Query())
	if len(embeddedRelations) > 0 && len(results) > 0 {
		results = resolveEmbeddedRelations(pool, tx, table, results, embeddedRelations)
	}

	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "JSON Encoder error", http.StatusInternalServerError)
	}
}

func validatePayload(pool *pgxpool.Pool, table, tenantID string, payload map[string]interface{}) error {
	rows, err := pool.Query(context.Background(), `
		SELECT column_name, rules FROM column_metadata 
		WHERE table_name = $1 AND tenant_id = $2
	`, table, tenantID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var col string
		var rulesMap map[string]interface{}
		var rulesJSON []byte
		if err := rows.Scan(&col, &rulesJSON); err != nil {
			continue
		}
		json.Unmarshal(rulesJSON, &rulesMap)

		if val, ok := payload[col]; ok {
			if err := validator.Validate(col, val, rulesMap); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolveEmbeddedRelations expands nested objects/arrays in results.
// It uses authPool for metadata lookups (FK detection) since the transaction might be restricted.
func resolveEmbeddedRelations(authPool *pgxpool.Pool, tx pgx.Tx, parentTable string, results []map[string]interface{}, relations []string) []map[string]interface{} {
	ctx := context.Background()
	log.Printf("[Expansion] Resolving relations for %s: %v", parentTable, relations)

	var parentIDs []interface{}
	for _, row := range results {
		if id, ok := row["id"]; ok {
			parentIDs = append(parentIDs, id)
		}
	}

	for _, relTable := range relations {
		// 1. One-to-Many Expansion (Children)
		fkCol := findFKColumn(ctx, authPool, relTable, parentTable)
		if fkCol != "" && len(parentIDs) > 0 {
			log.Printf("[Expansion] Found Child FK column '%s' in table '%s' referencing '%s'", fkCol, relTable, parentTable)
			sRelTable := pgx.Identifier{relTable}.Sanitize()
			sFKCol := pgx.Identifier{fkCol}.Sanitize()
			childQ := psql.Select("*").From(sRelTable).Where(squirrel.Eq{sFKCol: parentIDs})
			childSQL, childArgs, _ := childQ.ToSql()
			childRows, err := tx.Query(ctx, childSQL, childArgs...)
			if err == nil {
				childByParent := make(map[string][]map[string]interface{})
				for childRows.Next() {
					rowMap := scanRowToMap(childRows)
					if fkVal, ok := rowMap[fkCol]; ok {
						fkStr := fmt.Sprintf("%v", fkVal)
						childByParent[fkStr] = append(childByParent[fkStr], rowMap)
					}
				}
				childRows.Close()
				for i := range results {
					pid := fmt.Sprintf("%v", results[i]["id"])
					results[i][relTable] = childByParent[pid]
					if results[i][relTable] == nil {
						results[i][relTable] = []map[string]interface{}{}
					}
				}
			} else {
				log.Printf("[Expansion] Child query failed: %v", err)
			}
			continue
		}

		// 2. Many-to-One Expansion (Parent)
		parentFKCol := findFKColumn(ctx, authPool, parentTable, relTable)
		log.Printf("[Expansion] Parent lookup: findFKColumn(%s, %s) -> %s", parentTable, relTable, parentFKCol)
		if parentFKCol != "" {
			var targetIDs []interface{}
			for _, row := range results {
				if val, ok := row[parentFKCol]; ok && val != nil {
					targetIDs = append(targetIDs, val)
				}
			}
			log.Printf("[Expansion] Found %d target IDs for parent expansion", len(targetIDs))
			if len(targetIDs) > 0 {
				sRelTable := pgx.Identifier{relTable}.Sanitize()
				parentQ := psql.Select("*").From(sRelTable).Where(squirrel.Eq{"id": targetIDs})
				parentSQL, parentArgs, _ := parentQ.ToSql()
				log.Printf("[Expansion] Executing parent query: %s with args %v", parentSQL, parentArgs)
				parentRows, err := tx.Query(ctx, parentSQL, parentArgs...)
				if err == nil {
					parentsByID := make(map[string]map[string]interface{})
					for parentRows.Next() {
						rowMap := scanRowToMap(parentRows)
						if id, ok := rowMap["id"]; ok {
							parentsByID[fmt.Sprintf("%v", id)] = rowMap
						}
					}
					parentRows.Close()
					log.Printf("[Expansion] Found %d parents in DB", len(parentsByID))
					for i := range results {
						if fkVal, ok := results[i][parentFKCol]; ok && fkVal != nil {
							fkStr := fmt.Sprintf("%v", fkVal)
							if parentData, found := parentsByID[fkStr]; found {
								results[i][relTable] = parentData
							} else {
								log.Printf("[Expansion] Parent data not found for ID %s", fkStr)
							}
						}
					}
				} else {
					log.Printf("[Expansion] Parent query failed: %v", err)
				}
			}
		} else {
			log.Printf("[Expansion] No FK column found from %s to %s", parentTable, relTable)
		}
	}

	return results
}

func scanRowToMap(rows pgx.Rows) map[string]interface{} {
	v, err := rows.Values()
	if err != nil {
		return nil
	}
	fieldDescriptions := rows.FieldDescriptions()
	rowMap := make(map[string]interface{})
	for i, fd := range fieldDescriptions {
		key := string(fd.Name)
		val := v[i]

		// Handle UUID
		if fd.DataTypeOID == 2950 {
			if byteVal, ok := val.([16]uint8); ok {
				if parsed, err := uuid.FromBytes(byteVal[:]); err == nil {
					val = parsed.String()
				}
			} else if byteSlice, ok := val.([]byte); ok {
				if parsed, err := uuid.FromBytes(byteSlice); err == nil {
					val = parsed.String()
				}
			}
		}
		// Handle JSONB
		if fd.DataTypeOID == 3802 {
			if byteVal, ok := val.([]byte); ok {
				var jsonObj interface{}
				if err := json.Unmarshal(byteVal, &jsonObj); err == nil {
					val = jsonObj
				}
			} else if strVal, ok := val.(string); ok {
				var jsonObj interface{}
				if err := json.Unmarshal([]byte(strVal), &jsonObj); err == nil {
					val = jsonObj
				}
			}
		}
		rowMap[key] = val
	}
	return rowMap
}

func findFKColumn(ctx context.Context, pool *pgxpool.Pool, sourceTable, targetTable string) string {
	query := `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_name = $1
			AND ccu.table_name = $2
			AND tc.table_schema = 'public'
		LIMIT 1;
	`
	var colName string
	err := pool.QueryRow(ctx, query, sourceTable, targetTable).Scan(&colName)
	if err != nil {
		return ""
	}
	return colName
}

func findTargetTableFromFK(ctx context.Context, pool *pgxpool.Pool, table, column string) string {
	query := `
		SELECT ccu.table_name
		FROM information_schema.key_column_usage kcu
		JOIN information_schema.constraint_column_usage ccu
			ON kcu.constraint_name = ccu.constraint_name AND kcu.table_schema = ccu.table_schema
		WHERE kcu.table_name = $1 AND kcu.column_name = $2
			AND kcu.table_schema = 'public'
		LIMIT 1;
	`
	var targetTable string
	err := pool.QueryRow(ctx, query, table, column).Scan(&targetTable)
	if err != nil {
		return ""
	}
	return targetTable
}

func handleRecursiveInsert(ctx context.Context, authPool *pgxpool.Pool, tx pgx.Tx, table, tenantID string, payload map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("[RecursiveInsert] Processing table: %s, payload: %+v", table, payload)
	newPayload := make(map[string]interface{})
	for k, v := range payload {
		if nested, ok := v.(map[string]interface{}); ok {
			targetTable := findTargetTableFromFK(ctx, authPool, table, k)
			log.Printf("[RecursiveInsert] Key '%s' is nested. TargetTable from FK: %s", k, targetTable)
			if targetTable != "" {
				// Recursive insert
				insertedPayload, err := handleRecursiveInsert(ctx, authPool, tx, targetTable, tenantID, nested)
				if err != nil {
					return nil, err
				}
				id, err := performAtomicInsert(ctx, tx, targetTable, tenantID, insertedPayload)
				if err != nil {
					return nil, err
				}
				log.Printf("[RecursiveInsert] Linked record created in '%s' with ID: %s", targetTable, id)
				newPayload[k] = id
				continue
			}
		}
		newPayload[k] = v
	}
	return newPayload, nil
}

func performAtomicInsert(ctx context.Context, tx pgx.Tx, table, tenantID string, payload map[string]interface{}) (string, error) {
	q := psql.Insert(pgx.Identifier{table}.Sanitize())
	var cols []string
	var vals []interface{}

	cols = append(cols, "tenant_id")
	vals = append(vals, tenantID)

	for k, v := range payload {
		cols = append(cols, pgx.Identifier{k}.Sanitize())
		if mapVal, ok := v.(map[string]interface{}); ok {
			b, _ := json.Marshal(mapVal)
			vals = append(vals, string(b))
		} else {
			vals = append(vals, v)
		}
	}
	q = q.Columns(cols...).Values(vals...).Suffix("RETURNING id")
	sql, args, err := q.ToSql()
	if err != nil {
		return "", err
	}

	var id string
	err = tx.QueryRow(ctx, sql, args...).Scan(&id)
	return id, err
}

func HandleDynamicPost(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	table := chi.URLParam(r, "table")
	claims, _ := r.Context().Value(UserContextKey).(map[string]interface{})
	tenantID, _ := claims["tenant_id"].(string)
	userEmail, _ := claims["email"].(string)

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid syntax", http.StatusBadRequest)
		return
	}

	if err := validatePayload(pool, table, tenantID, payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Transaction error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	// Recursion
	finalPayload, err := handleRecursiveInsert(context.Background(), pool, tx, table, tenantID, payload)
	if err != nil {
		http.Error(w, "Nested insert failed", http.StatusInternalServerError)
		return
	}

	// Security context
	tx.Exec(context.Background(), "SELECT set_config('role', 'gobase_api_user', true)")
	tx.Exec(context.Background(), "SELECT set_config('app.tenant_id', $1, true)", tenantID)

	q := psql.Insert(pgx.Identifier{table}.Sanitize())
	var cols []string
	var vals []interface{}

	cols = append(cols, "tenant_id", "created_by", "modified_by")
	vals = append(vals, tenantID, userEmail, userEmail)

	for k, v := range finalPayload {
		cols = append(cols, pgx.Identifier{k}.Sanitize())
		if mv, ok := v.(map[string]interface{}); ok {
			b, _ := json.Marshal(mv)
			vals = append(vals, string(b))
		} else {
			vals = append(vals, v)
		}
	}

	sql, args, _ := q.Columns(cols...).Values(vals...).Suffix("RETURNING id").ToSql()
	var newID string
	if err := tx.QueryRow(context.Background(), sql, args...).Scan(&newID); err != nil {
		http.Error(w, "Insert failed", http.StatusInternalServerError)
		return
	}

	tx.Commit(context.Background())
	RecordAuditLog(pool, tenantID, userEmail, table, "INSERT", payload)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"id": newID})
}

func HandleDynamicPut(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	table := chi.URLParam(r, "table")
	id := chi.URLParam(r, "id")
	claims, _ := r.Context().Value(UserContextKey).(map[string]interface{})
	tenantID, _ := claims["tenant_id"].(string)
	userEmail, _ := claims["email"].(string)

	var payload map[string]interface{}
	json.NewDecoder(r.Body).Decode(&payload)

	if err := validatePayload(pool, table, tenantID, payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Transaction error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	// Recursion also for PUT (supports linked creation)
	finalPayload, err := handleRecursiveInsert(context.Background(), pool, tx, table, tenantID, payload)
	if err != nil {
		http.Error(w, "Nested update failed", http.StatusInternalServerError)
		return
	}

	tx.Exec(context.Background(), "SELECT set_config('role', 'gobase_api_user', true)")
	tx.Exec(context.Background(), "SELECT set_config('app.tenant_id', $1, true)", tenantID)

	q := psql.Update(pgx.Identifier{table}.Sanitize()).Where(squirrel.Eq{"id": id}).
		Set("modified_at", squirrel.Expr("NOW()")).
		Set("modified_by", userEmail)

	for k, v := range finalPayload {
		sCol := pgx.Identifier{k}.Sanitize()
		if mv, ok := v.(map[string]interface{}); ok {
			b, _ := json.Marshal(mv)
			q = q.Set(sCol, string(b))
		} else {
			q = q.Set(sCol, v)
		}
	}

	sql, args, _ := q.ToSql()
	if _, err := tx.Exec(context.Background(), sql, args...); err != nil {
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}

	tx.Commit(context.Background())
	RecordAuditLog(pool, tenantID, userEmail, table, "UPDATE", payload)
	w.WriteHeader(http.StatusOK)
}

func HandleDynamicDelete(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, hub *realtime.Hub) {
	table := chi.URLParam(r, "table")
	id := chi.URLParam(r, "id")
	claims, _ := r.Context().Value(UserContextKey).(map[string]interface{})
	tenantID, _ := claims["tenant_id"].(string)
	userEmail, _ := claims["email"].(string)

	q := psql.Delete(pgx.Identifier{table}.Sanitize()).Where(squirrel.Eq{"id": id})
	sql, args, _ := q.ToSql()

	tx, err := pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Transaction error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	tx.Exec(context.Background(), "SELECT set_config('role', 'gobase_api_user', true)")
	tx.Exec(context.Background(), "SELECT set_config('app.tenant_id', $1, true)", tenantID)

	if _, err := tx.Exec(context.Background(), sql, args...); err != nil {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}

	tx.Commit(context.Background())
	RecordAuditLog(pool, tenantID, userEmail, table, "DELETE", map[string]interface{}{"id": id})
	w.WriteHeader(http.StatusNoContent)
}
