package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
)

func TestGrpcCode(t *testing.T) {
	assert.Equal(t, codes.OK, Code(nil), "code should be OK")

	err := fmt.Errorf("test error")
	assert.Equal(t, codes.Unknown, Code(err), "code should be unknown")

	err = WithCode(err, codes.InvalidArgument)
	assert.Equal(t, codes.InvalidArgument, Code(err), "code should be InvalidArgument")

	err = WithCode(err, codes.AlreadyExists)
	assert.Equal(t, codes.AlreadyExists, Code(err), "code should be AlreadyExists")

	err = WrapPrefix(err, "wrapped", 0)
	assert.Equal(t, codes.AlreadyExists, Code(err), "code should still be AlreadyExists")
}

func TestHttpStatusCode(t *testing.T) {
	assert.Equal(t, 200, HTTPStatusCode(nil), "non errors should 200")

	err := fmt.Errorf("test error")
	assert.Equal(t, 500, HTTPStatusCode(err), "should default to 500")

	err = WithCode(err, codes.FailedPrecondition)
	assert.Equal(t, 412, HTTPStatusCode(err), "GRPC error should map to 412 http error")

	err = WithHTTPStatusCode(err, 409)
	assert.Equal(t, 409, HTTPStatusCode(err), "http status code should override grpc code")

	err = WrapPrefix(err, "wrapped", 0)
	assert.Equal(t, 409, HTTPStatusCode(err), "http status code should still be 409")
}

func TestPrefix(t *testing.T) {
	err := fmt.Errorf("test error")
	err = WrapPrefix(err, "wrapped", 0)
	assert.Equal(t, "wrapped: test error", err.Error(), "error should have prefix")
}

func TestGRPCStatus(t *testing.T) {
	badRequest := &errdetails.BadRequest{
		FieldViolations: []*errdetails.BadRequest_FieldViolation{
			{
				Field:       "test_field",
				Description: "Test field was empty",
			},
		},
	}

	err := NewC("test error", codes.InvalidArgument).WithDetails(badRequest)
	st := err.GRPCStatus()
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Equal(t, "test error", st.Message())
	assert.Equal(t, "test_field", st.Details()[0].(*errdetails.BadRequest).FieldViolations[0].Field)
}

func TestPublicMessage(t *testing.T) {
	err := New("test error")
	assert.Equal(t, "test error", err.GRPCStatus().Message())

	err = err.WithUserPresentableMessage("public message")
	assert.Equal(t, "public message", err.GRPCStatus().Message())
}

func TestWrappedError(t *testing.T) {
	err := NewC("test error", codes.InvalidArgument)
	wrappedErr := fmt.Errorf("%w : wrapped error", err)

	assert.Equal(t, codes.InvalidArgument, Code(wrappedErr))
}

func TestMark(t *testing.T) {
	err := NewC("test error", codes.InvalidArgument)
	markedErr := Mark(err, 0)

	assert.True(t, Is(markedErr, err), "Marked error should still satisfy Is")
	assert.Equal(t, codes.InvalidArgument, Code(markedErr))
}

func TestLogFields(t *testing.T) {
	// Test adding a single field
	err := New("test error").WithLogField("user_id", "123")
	fields := err.LogFields()
	assert.NotNil(t, fields, "fields should not be nil")
	assert.Equal(t, "123", fields["user_id"], "user_id field should be set")

	// Test adding another field
	err = err.WithLogField("action", "create")
	fields = err.LogFields()
	assert.Equal(t, "123", fields["user_id"], "user_id field should still be set")
	assert.Equal(t, "create", fields["action"], "action field should be set")

	// Test WithLogFields with multiple fields
	err2 := New("another error").WithLogFields(map[string]interface{}{
		"order_id": "ord_123",
		"amount":   100,
		"currency": "USD",
	})
	fields2 := err2.LogFields()
	assert.Equal(t, "ord_123", fields2["order_id"])
	assert.Equal(t, 100, fields2["amount"])
	assert.Equal(t, "USD", fields2["currency"])
}

func TestLogFieldsNil(t *testing.T) {
	// Test that LogFields returns nil when no fields have been added
	err := New("test error")
	fields := err.LogFields()
	assert.Nil(t, fields, "fields should be nil when none added")
}

func TestLogFieldsPreservedThroughWrapPrefix(t *testing.T) {
	// Test that log fields are preserved when using WrapPrefix
	err := New("test error").WithLogField("request_id", "req_123")
	wrapped := WrapPrefix(err, "wrapped", 0)

	fields := wrapped.LogFields()
	assert.NotNil(t, fields, "fields should be preserved")
	assert.Equal(t, "req_123", fields["request_id"], "request_id should be preserved")
}

func TestLogFieldsPreservedThroughMark(t *testing.T) {
	// Test that log fields are preserved when using Mark
	err := New("test error").
		WithLogField("user_id", "usr_123").
		WithLogField("trace_id", "trace_456")
	marked := Mark(err, 0)

	fields := marked.LogFields()
	assert.NotNil(t, fields, "fields should be preserved")
	assert.Equal(t, "usr_123", fields["user_id"], "user_id should be preserved")
	assert.Equal(t, "trace_456", fields["trace_id"], "trace_id should be preserved")
}

func TestPackageLevelWithLogField(t *testing.T) {
	// Test package-level WithLogField helper
	err := fmt.Errorf("standard error")
	wrappedErr := WithLogField(err, "component", "database")

	fields := wrappedErr.LogFields()
	assert.NotNil(t, fields, "fields should be set")
	assert.Equal(t, "database", fields["component"])
}

func TestPackageLevelWithLogFields(t *testing.T) {
	// Test package-level WithLogFields helper
	err := fmt.Errorf("standard error")
	wrappedErr := WithLogFields(err, map[string]interface{}{
		"retry_count": 3,
		"timeout_ms":  5000,
	})

	fields := wrappedErr.LogFields()
	assert.NotNil(t, fields, "fields should be set")
	assert.Equal(t, 3, fields["retry_count"])
	assert.Equal(t, 5000, fields["timeout_ms"])
}

func TestLogFieldsChaining(t *testing.T) {
	// Test that chaining works with other methods
	err := New("test error").
		WithCode(codes.InvalidArgument).
		WithLogField("user_id", "123").
		WithHTTPStatusCode(400).
		WithLogField("validation_error", "email_invalid").
		WithUserPresentableMessage("Invalid email address")

	assert.Equal(t, codes.InvalidArgument, err.Code())
	assert.Equal(t, 400, err.HTTPStatusCode())
	assert.Equal(t, "Invalid email address", err.UserPresentableMessage())

	fields := err.LogFields()
	assert.Equal(t, "123", fields["user_id"])
	assert.Equal(t, "email_invalid", fields["validation_error"])
}
