package logger_test

import (
	"bytes"
	"testing"

	"gobase/internal/logger"

	"github.com/stretchr/testify/assert"
)

func TestInitLogger(t *testing.T) {
	var buf bytes.Buffer
	l := logger.InitLogger(&buf, "info")

	l.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, `"level":"INFO"`)
}
