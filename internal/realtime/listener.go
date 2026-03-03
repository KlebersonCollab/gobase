package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgEvent represents the JSON payload dispatched by out triggers
type PgEvent struct {
	Table    string      `json:"table"`
	Action   EventType   `json:"action"`
	TenantID string      `json:"tenant_id"`
	Data     interface{} `json:"data"` // The row payload
}

// StartListener connects directly to PG and issues LISTEN gobase_events
func StartListener(ctx context.Context, pool *pgxpool.Pool, hub *Hub) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Realtime listener shutting down...")
				return
			default:
				listenBlocks(ctx, pool, hub)
				log.Println("Realtime listener disconnected. Retrying in 5 seconds...")
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

func listenBlocks(ctx context.Context, pool *pgxpool.Pool, hub *Hub) {
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		log.Printf("Failed to acquire connection for LISTEN: %v\n", err)
		return
	}
	defer conn.Release()

	// Issue the LISTEN command on the specific channel
	_, err = conn.Exec(ctx, "LISTEN gobase_events")
	if err != nil {
		log.Printf("Failed to LISTEN: %v\n", err)
		return
	}
	log.Println("Successfully listening to PG channel: gobase_events")

	// Block and wait for notifications
	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, exit cleanly
			}
			log.Printf("Error waiting for notification: %v\n", err)
			return // Breaking out triggers the wrapper retry
		}

		var event PgEvent
		if err := json.Unmarshal([]byte(notification.Payload), &event); err != nil {
			log.Printf("Failed to parse PG notification: %v", err)
			continue
		}
		log.Printf("[LISTENER] Received PG Event: %s on %s for tenant %s\n", event.Action, event.Table, event.TenantID)

		// Dispatch to the WS Hub
		hub.Broadcast <- &BroadcastMessage{
			TenantID: event.TenantID,
			Table:    event.Table,
			Action:   event.Action,
			Data:     event.Data,
		}
	}
}

// EnsureBroadcastFunction creates the shared trigger function that emits JSON notifications
func EnsureBroadcastFunction(pool *pgxpool.Pool) error {
	const sql = `
	CREATE OR REPLACE FUNCTION gobase_broadcast_event() RETURNS TRIGGER AS $$
	DECLARE
		payload JSONB;
	BEGIN
		payload = jsonb_build_object(
			'table', TG_TABLE_NAME,
			'action', TG_OP,
			'tenant_id', CASE WHEN TG_OP = 'DELETE' THEN OLD.tenant_id ELSE NEW.tenant_id END,
			'data', CASE 
				WHEN TG_OP = 'DELETE' THEN row_to_json(OLD)::jsonb
				ELSE row_to_json(NEW)::jsonb
			END
		);
		PERFORM pg_notify('gobase_events', payload::text);
		RETURN COALESCE(NEW, OLD);
	END;
	$$ LANGUAGE plpgsql;
	`
	_, err := pool.Exec(context.Background(), sql)
	return err
}

// ApplyRealtimeTrigger attaches the broadcast trigger to a specific table
func ApplyRealtimeTrigger(pool *pgxpool.Pool, tableName string) error {
	// We use a separate EXEC for dropping to avoid errors if it doesn't exist, though CREATE OR REPLACE isn't for triggers
	triggerName := "gobase_realtime_trigger"
	pool.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", triggerName, tableName))

	sql := fmt.Sprintf(`
		CREATE TRIGGER %s
		AFTER INSERT OR UPDATE OR DELETE ON %s
		FOR EACH ROW EXECUTE FUNCTION gobase_broadcast_event();
	`, triggerName, tableName)

	_, err := pool.Exec(context.Background(), sql)
	return err
}
