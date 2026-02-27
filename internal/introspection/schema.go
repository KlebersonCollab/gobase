package introspection

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Schema represents the entire cached database structure
type Schema struct {
	Tables map[string]*Table
}

// Table represents a database table and its columns
type Table struct {
	Name    string
	Columns map[string]*Column
}

// Column represents a specific column inside a table
type Column struct {
	Name       string
	DataType   string
	IsNullable bool
}

// LoadSchema connects to Postgres and reads the information_schema to build the memory map
func LoadSchema(ctx context.Context, pool *pgxpool.Pool) (*Schema, error) {
	query := `
		SELECT 
			t.table_name,
			c.column_name,
			c.data_type,
			c.is_nullable
		FROM 
			information_schema.tables t
		JOIN 
			information_schema.columns c ON t.table_name = c.table_name
		WHERE 
			t.table_schema = 'public'
			AND t.table_type = 'BASE TABLE'
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query information_schema: %w", err)
	}
	defer rows.Close()

	schema := &Schema{
		Tables: make(map[string]*Table),
	}

	for rows.Next() {
		var tableName, colName, dataType, isNullable string
		if err := rows.Scan(&tableName, &colName, &dataType, &isNullable); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Ensure table exists in our map
		table, exists := schema.Tables[tableName]
		if !exists {
			table = &Table{
				Name:    tableName,
				Columns: make(map[string]*Column),
			}
			schema.Tables[tableName] = table
		}

		// Add column to the table
		table.Columns[colName] = &Column{
			Name:       colName,
			DataType:   dataType,
			IsNullable: isNullable == "YES",
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading rows: %w", err)
	}

	return schema, nil
}
