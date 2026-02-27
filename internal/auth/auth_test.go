package auth_test

import (
	"context"
	"os"
	"testing"

	"gobase/internal/auth"
	"gobase/internal/database"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthFlow(t *testing.T) {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		t.Skip("Skipping integration test: DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// 1. Setup tables
	pool.Exec(ctx, "DROP TABLE IF EXISTS user_tenants CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS collection_permissions CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS refresh_tokens CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS users CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS tenants CASCADE")

	err = auth.SetupAuthTables(ctx, pool)
	require.NoError(t, err)

	defer func() {
		pool.Exec(ctx, "DROP TABLE IF EXISTS user_tenants CASCADE")
		pool.Exec(ctx, "DROP TABLE IF EXISTS collection_permissions CASCADE")
		pool.Exec(ctx, "DROP TABLE IF EXISTS refresh_tokens CASCADE")
		pool.Exec(ctx, "DROP TABLE IF EXISTS users CASCADE")
		pool.Exec(ctx, "DROP TABLE IF EXISTS tenants CASCADE")
	}()

	// 2. Register (create new tenant)
	user, tenant, err := auth.RegisterNewTenant(ctx, pool, "test@gobase.com", "mysecurepassword", "Acme Corp")
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.NotEmpty(t, tenant.TenantID)
	assert.Equal(t, "admin", tenant.Role)
	assert.Equal(t, "test@gobase.com", user.Email)

	// 3. Login Failed
	_, _, err = auth.Login(ctx, pool, "test@gobase.com", "wrongpassword")
	assert.Error(t, err)

	// 4. Login Success
	loggedUser, tenants, err := auth.Login(ctx, pool, "test@gobase.com", "mysecurepassword")
	require.NoError(t, err)
	require.Len(t, tenants, 1)
	assert.Equal(t, "admin", tenants[0].Role)

	// 5. Generate token and verify structure
	token, err := auth.GenerateAccessToken(loggedUser, tenants[0].TenantID, tenants[0].Role)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return auth.JwtSecret, nil
	})
	require.NoError(t, err)
	claims := parsed.Claims.(jwt.MapClaims)

	assert.Equal(t, loggedUser.ID, claims["sub"])
	assert.Equal(t, tenants[0].TenantID, claims["tenant_id"])
	assert.Equal(t, "admin", claims["role"])

	// 6. Register join tenant (member with 0 access)
	memberUser, memberTenant, err := auth.RegisterJoinTenant(ctx, pool, "member@gobase.com", "memberpass", "Acme Corp")
	require.NoError(t, err)
	assert.Equal(t, "member", memberTenant.Role)

	// 7. Check permission (member has no permissions)
	allowed, err := auth.CheckPermission(ctx, pool, memberUser.ID, memberTenant.TenantID, "some_table", "read")
	require.NoError(t, err)
	assert.False(t, allowed)

	// 8. Admin bypasses permission checks
	allowed, err = auth.CheckPermission(ctx, pool, user.ID, tenant.TenantID, "any_table", "read")
	require.NoError(t, err)
	assert.True(t, allowed)
}
