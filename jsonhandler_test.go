package prefab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
)

func TestJSONHandler(t *testing.T) {
	customHandler := func(req *http.Request) (any, error) {
		return map[string]string{
			"method": req.Method,
			"url":    req.URL.String(),
		}, nil
	}

	httpHandler := wrapJSONHandler(customHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	rr := httptest.NewRecorder()
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"method":"GET","url":"/test"}`, rr.Body.String())
}

func TestJSONHandlerError(t *testing.T) {
	customHandler := func(req *http.Request) (any, error) {
		return nil, errors.NewC("test error", codes.Internal)
	}

	httpHandler := wrapJSONHandler(customHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(logging.EnsureLogger(context.Background()))
	rr := httptest.NewRecorder()
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.JSONEq(t, `{"code":13,"codeName":"INTERNAL","message":"test error", "details": []}`, rr.Body.String())
}
