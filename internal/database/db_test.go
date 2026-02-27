package database

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConnectDB(t *testing.T) {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		t.Skip("Skipping test because DATABASE_URL is not set")
	}

	db, err := Connect(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, db)

	err = db.Ping(context.Background())
	if err == nil {
		db.Close()
	}
}
