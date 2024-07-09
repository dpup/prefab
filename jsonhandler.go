package prefab

import (
	"encoding/json"
	"net/http"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/proto"
)

// JSONHandler are regular HTTP handlers that return a response that should be
// encoded in a similar fashion to a gRPC Gateway response.
//
// If the return value is a proto.Message, it will be marshaled using the same
// JSON marshaler as the gRPC Gateway.
type JSONHandler func(req *http.Request) (any, error)

func wrapJSONHandler(fn JSONHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := execJSONHandler(fn, w, r)
		if err != nil {
			// TODO: Log warning and error based on status code.
			logging.Errorw(r.Context(), "JSON handler error", "error", err,
				"req.method", r.Method, "req.url", r.URL.String())

			c := int32(errors.Code(err))
			b, ferr := JSONMarshalOptions.Marshal(&CustomErrorResponse{
				Code:     c,
				CodeName: code.Code_name[c],
				Message:  err.Error(),
			})
			if ferr != nil {
				http.Error(w, "error encoding response", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(errors.HTTPStatusCode(err))
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	})
}

func execJSONHandler(fn JSONHandler, w http.ResponseWriter, r *http.Request) error {
	// Execute the handler.
	resp, err := fn(r)
	if err != nil {
		return err
	}

	// If the response is a proto.Message, marshal it using the JSON marshaler.
	var b []byte
	if pb, ok := resp.(proto.Message); ok {
		b, err = JSONMarshalOptions.Marshal(pb)
	} else {
		b, err = json.Marshal(resp)
	}
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)

	return nil
}
