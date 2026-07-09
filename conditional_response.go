package prefab

import "net/http"

// conditionalResponse wraps a handler to enforce HTTP conditional-response
// hygiene: when a handler emits a 304 Not Modified (typically via
// serverutil.SendStatusCode, e.g. from the etag plugin), RFC 7232 requires the
// response to carry no message body. The GRPC Gateway still marshals the
// response message, so without this the 304 would carry a stray `{}` body and
// content headers.
//
// It is applied only to the Gateway mux, whose responses are unary and never
// streamed, so the wrapper does not need to forward Flush/Hijack. Streaming
// endpoints (e.g. SSE) are mounted separately and are unaffected.
func conditionalResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&conditionalWriter{ResponseWriter: w}, r)
	})
}

// conditionalWriter suppresses the body of a 304 Not Modified response and
// removes the content headers that would otherwise describe a body. Absorbing
// the body write (rather than letting net/http reject it with
// ErrBodyNotAllowed) also keeps the gzip layer from stamping a Content-Encoding
// on an empty response and avoids a spurious grpclog error.
type conditionalWriter struct {
	http.ResponseWriter
	wroteHeader  bool
	suppressBody bool
}

func (w *conditionalWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		if code == http.StatusNotModified {
			w.suppressBody = true
			h := w.Header()
			h.Del("Content-Type")
			h.Del("Content-Length")
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *conditionalWriter) Write(b []byte) (int, error) {
	if w.suppressBody {
		// Report the bytes as written but drop them; a 304 has no body.
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}
