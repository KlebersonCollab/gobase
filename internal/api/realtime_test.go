package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gobase/internal/api"
	"gobase/internal/auth"
	"gobase/internal/database"
	"gobase/internal/realtime"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealtimeWebsockets(t *testing.T) {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		t.Skip("Skipping integration test: DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx)
	require.NoError(t, err)
	defer pool.Close()

	// Clean tables
	pool.Exec(ctx, "DROP TABLE IF EXISTS websocket_test CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS collection_permissions CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS user_tenants CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS refresh_tokens CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS users CASCADE")
	pool.Exec(ctx, "DROP TABLE IF EXISTS tenants CASCADE")

	err = auth.SetupAuthTables(ctx, pool)
	require.NoError(t, err)

	hub := realtime.NewHub()
	go hub.Run()

	router := api.NewRouterWithHub(pool, hub)

	server := httptest.NewServer(router)
	defer server.Close()

	// Generate Test User (admin)
	user, tenant, err := auth.RegisterNewTenant(ctx, pool, "ws_user@acme.com", "password", "WS_Acme")
	require.NoError(t, err)
	token, err := auth.GenerateAccessToken(user, tenant.TenantID, tenant.Role)
	require.NoError(t, err)

	// Create a test table via API
	createTablePayload := map[string]interface{}{"name": "websocket_test"}
	b, _ := json.Marshal(createTablePayload)
	req, _ := http.NewRequest("POST", server.URL+"/api/schema/tables", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	res, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, res.StatusCode)

	// 2. Connect via WebSockets to our Hub
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/realtime"

	header := http.Header{}
	header.Add("Authorization", "Bearer "+token)

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	require.NoError(t, err)
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)

	// 3. Trigger a POST request to insert a record
	insertPayload := map[string]interface{}{}
	insertPayload["data"] = map[string]interface{}{"note": "hello ws!"}
	b, _ = json.Marshal(insertPayload)
	req, _ = http.NewRequest("POST", server.URL+"/api/collections/websocket_test", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+token)

	res, err = client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, res.StatusCode)

	// 4. Verify we actually get a message on the WebSocket!
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := ws.ReadMessage()
	require.NoError(t, err)

	var output realtime.OutputMessage
	err = json.Unmarshal(msg, &output)
	require.NoError(t, err)

	assert.Equal(t, "websocket_test", output.Table)
	assert.Equal(t, realtime.EventInsert, output.Action)
}
