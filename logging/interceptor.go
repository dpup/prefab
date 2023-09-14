package logging

import (
	"context"

	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc"
)

// Interceptor returns a GRPC Logging interceptor configured to log using
// the prefab logging adapter.
func Interceptor() grpc.UnaryServerInterceptor {
	return grpc_logging.UnaryServerInterceptor(grpc_logging.LoggerFunc(func(ctx context.Context, lvl grpc_logging.Level, msg string, fields ...any) {
		logger := FromContext(ctx)
		for i := 0; i < len(fields); i += 2 {
			key := fields[i].(string)
			value := fields[i+1]
			// TODO: This might be too inefficient.
			logger = logger.With(key, value)
		}

		switch lvl {
		case grpc_logging.LevelDebug:
			logger.Debug(msg)
		case grpc_logging.LevelInfo:
			logger.Info(msg)
		case grpc_logging.LevelWarn:
			logger.Warn(msg)
		case grpc_logging.LevelError:
			logger.Error(msg)
		default:
			logger.Panicf("unknown log level %v", lvl)
		}
	}))
}
