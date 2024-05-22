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
