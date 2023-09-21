package server

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/types/known/anypb"
)

// gatewayErrorHandler overrides the default error handling, to update the
// structure of the error response. The default error handler handles writing of
// headers and other tasks, so we intercept at the marshaling step.
//
// Default error handler: https://github.com/grpc-ecosystem/grpc-gateway/blob/0e7b2ebe117212ae651f0370d2753f237799afdf/runtime/errors.go#L93
// Proto representation of GRPC status which is returned by default: https://pkg.go.dev/google.golang.org/genproto/googleapis/rpc/status
func gatewayErrorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	m := &monkeypatcher{Marshaler: marshaler}
	runtime.DefaultHTTPErrorHandler(ctx, mux, m, w, r, err)
}

// monkeyPatcher wraps a GRPC Gateway Marshaller and hijacks the marshalling
// of the grpc Status proto, to output our own error type.
type monkeypatcher struct {
	runtime.Marshaler
}

func (m *monkeypatcher) Marshal(v interface{}) ([]byte, error) {
	if s, ok := v.(grpcStatusProto); ok {
		v = &customErrorResponse{
			Code:     s.GetCode(),
			CodeName: code.Code_name[s.GetCode()],
			Message:  s.GetMessage(),
			Details:  s.GetDetails(),
		}
	}
	return m.Marshaler.Marshal(v)
}

// Satisfies the interface exposed by the GRPC status proto, which in the
// context of the GRPC Gateway is a private type.
type grpcStatusProto interface {
	GetCode() int32
	GetMessage() string
	GetDetails() []*anypb.Any
}

type customErrorResponse struct {
	Code     int32
	CodeName string
	Message  string
	Details  []*anypb.Any
}
