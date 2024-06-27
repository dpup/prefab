package logging

import (
	"context"
	"reflect"

	"github.com/dpup/prefab/errors"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

const stackSize = 5

// Interceptor returns a GRPC Logging interceptor configured to log using
// the prefab logging adapter.
func Interceptor() grpc.UnaryServerInterceptor {
	return grpc_middleware.ChainUnaryServer(scopingInterceptor, grpcLoggingInterceptor, errorInterceptor)
}

// Creates a new logging scope for each request, adding the RPC method name as
// the logger name. This ensures logging.Track works as expected.
func scopingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	return handler(With(ctx, FromContext(ctx).Named(info.FullMethod)), req)
}

// Adds extra error fields to the logging context.
func errorInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		// Recover from panics, wrap them in an error so we can get a clean stack.
		if r := recover(); r != nil {
			Track(ctx, "error.panic", true)
			err = errors.Wrap(r, 3)
			resp = nil
		}

		// Track error information for use in logging interceptor.
		if err != nil {
			trackError(ctx, err)
		}
	}()

	resp, err = handler(ctx, req)
	return
}

func trackError(ctx context.Context, err error) {
	Track(ctx, "error.type", reflect.TypeOf(err))
	Track(ctx, "error.http_status", errors.HTTPStatusCode(err))

	// Add a minimalist stack trace to the log.
	var prefabErr *errors.Error
	if errors.As(err, &prefabErr) {
		Track(ctx, "error.stack_trace", prefabErr.MinimalStack(0, stackSize))
		Track(ctx, "error.original_type", prefabErr.TypeName())
	}
}

// Standard interceptor from the GRPC Logging middleware.
var grpcLoggingInterceptor = grpc_logging.UnaryServerInterceptor(grpc_logging.LoggerFunc(func(ctx context.Context, lvl grpc_logging.Level, msg string, fields ...any) {
	logger := FromContext(ctx)

	if z, ok := logger.(*ZapLogger); ok {
		// Disable zap's stack trace for all but panic level, because it's not very
		// helpful to have the stacktrace of the logger interceptor. Errors that
		// have stack traces will be added as fields, by the errorInterceptor.
		z.z = z.z.Desugar().WithOptions(
			zap.AddStacktrace(zapcore.PanicLevel),
		).Sugar()
	}

	for i := 0; i < len(fields); i += 2 {
		key, _ := fields[i].(string)
		value := fields[i+1]
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
