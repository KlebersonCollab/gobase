package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupStorageTables(ctx context.Context, pool *pgxpool.Pool) error {
	query := `
		CREATE TABLE IF NOT EXISTS storage_objects (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			filename TEXT NOT NULL,
			content_type TEXT NOT NULL,
			size BIGINT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		ALTER TABLE storage_objects ENABLE ROW LEVEL SECURITY;
		ALTER TABLE storage_objects FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS storage_isolation_policy ON storage_objects;
		CREATE POLICY storage_isolation_policy ON storage_objects
			USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
			WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);
		
		DO $$
		BEGIN
			-- Ensure role exists just in case
			IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'gobase_api_user') THEN
				CREATE ROLE gobase_api_user;
			END IF;
		END
		$$;
		GRANT ALL ON storage_objects TO gobase_api_user;
	`
	_, err := pool.Exec(ctx, query)
	return err
}
