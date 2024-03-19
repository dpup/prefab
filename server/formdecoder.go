package server

import (
	"fmt"
	"io"
	"net/url"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/v2/utilities"
	"google.golang.org/protobuf/proto"
)

// Forked from pending issue here:
// https://github.com/grpc-ecosystem/grpc-gateway/issues/7
type formDecoder struct {
	runtime.Marshaler
}

// ContentType means the content type of the response
func (u formDecoder) ContentType(_ interface{}) string {
	return "application/json"
}

func (u formDecoder) Marshal(v interface{}) ([]byte, error) {
	// can marshal the response in proto message format
	j := runtime.JSONPb{}
	return j.Marshal(v)
}

// NewDecoder indicates how to decode the request
func (u formDecoder) NewDecoder(r io.Reader) runtime.Decoder {
	return runtime.DecoderFunc(func(p interface{}) error {
		msg, ok := p.(proto.Message)
		if !ok {
			return fmt.Errorf("not proto message")
		}

		formData, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		values, err := url.ParseQuery(string(formData))
		if err != nil {
			return err
		}

		fmt.Println("frm data", string(formData))

		fmt.Println("values", values)

		filter := &utilities.DoubleArray{}

		err = runtime.PopulateQueryParameters(msg, values, filter)

		if err != nil {
			return err
		}

		return nil
	})
}
