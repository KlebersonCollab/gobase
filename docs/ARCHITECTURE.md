# GoBase Architecture

## The Vision
A single-binary Backend-as-a-Service written in Go that provides REST APIs, Realtime features, and secure Multi-tenancy directly on top of PostgreSQL.

## Core Pillars

1. **pgx & Dynamic SQL**:
   Instead of using ORMs like GORM which map directly to structs compiled inside the binary, GoBase uses `pgx` (Postgres driver) and dynamic SQL builders (like `squirrel`).
   When a user makes a `POST /api/collections/posts`, GoBase inspects the request body, checks it against the database schema (cached in memory), and dynamically builds a `INSERT INTO posts (...) VALUES (...)`.

2. **Zero Downtime Migrations (DDL)**:
   Developers can add tables and typed columns via API. Go translates these to Postgres DDL (`ALTER TABLE`). Because Postgres supports transactional DDL (`BEGIN; ALTER TABLE... COMMIT;`), we can alter schemas dynamically without corrupting data or requiring server restarts.

3. **Row-Level Security (RLS)**:
   This is the security backbone for multi-tenancy.
   Instead of appending `WHERE tenant_id = X` to every single query (which risks leaking data if someone forgets), we inject the tenant scope at the connection level.
   Before running the actual API query, the Go server uses the same transaction to do: `SET LOCAL "app.tenant_id" = 'uuid'`.
   The `posts` table has an RLS policy: `CREATE POLICY tenant_isolation ON posts USING (tenant_id = current_setting('app.tenant_id')::uuid)`. The database enforces the security at the lowest level.

4. **Real-time Engine**:
   When data changes, a Postgres trigger executes `NOTIFY gobase_events, '{ "table": "posts", "action": "INSERT", "record": {...} }'`.
   A single Go routine listens via `LISTEN gobase_events` connection.
   It unmarshals the JSON and broadcasts it to the connected Websockets (checking first if the Websocket's decoded JWT `tenant_id` matches the record's `tenant_id`).

5. **JSONB Fallback**:
   While tables can have strict types (UUID, VARCHAR, TIMESTAMP), they should typically include a `data JSONB` column. This gives the "NoSQL" feel where developers can save random unstructured fields without needing an `ALTER TABLE` for every small change. Postgres will transparently index and search within this JSONB.
