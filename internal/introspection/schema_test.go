package introspection_test

import (
	"context"
	"os"
	"testing"

	"gobase/internal/database"
	"gobase/internal/introspection"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntrospectSchema(t *testing.T) {
	// Require test database to be running
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		t.Skip("Skipping introspection test: DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// 1. Create a dummy table directly to test introspection
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_users (
			id UUID PRIMARY KEY,
			email VARCHAR(255) NOT NULL,
			created_at TIMESTAMP
		)
	`)
	require.NoError(t, err)

	defer func() {
		pool.Exec(ctx, "DROP TABLE test_users")
	}()

	// 2. Run the Introspection Engine
	schema, err := introspection.LoadSchema(ctx, pool)
	require.NoError(t, err)
	assert.NotNil(t, schema)

	// 3. Assert our table was found and mapped correctly
	table, exists := schema.Tables["test_users"]
	assert.True(t, exists, "Table test_users should be in the introspected schema")

	if exists {
		assert.Equal(t, "test_users", table.Name)

		idCol, hasId := table.Columns["id"]
		assert.True(t, hasId)
		assert.Equal(t, "uuid", idCol.DataType)

		emailCol, hasEmail := table.Columns["email"]
		assert.True(t, hasEmail)
		assert.Equal(t, "character varying", emailCol.DataType)
		assert.False(t, emailCol.IsNullable)
	}
}
