package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gobase/internal/api"

	"github.com/stretchr/testify/assert"
)

func TestCoreRouter(t *testing.T) {
	router := api.NewRouter(nil)

	// 1. Healthcheck should be wired
	req, _ := http.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// 2. Dynamic Collection Route Test (Not implemented yet, but router should map it)
	req, _ = http.NewRequest("GET", "/api/collections/users", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Will return 401 Unauthorized specifically handled by our JWT middleware
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
