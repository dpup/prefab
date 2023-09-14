package logging

import "context"

type ctxkey struct {
	logger Logger
}

// With attaches a logger to the context.
//
// This can be used to create logging scopes like so:
//
//	for _, u := range users {
//	  ctx := With(ctx, logger.Named(u.ID))
//	  processUser(ctx, u)
//	}
func With(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, ctxkey{}, &ctxkey{
		logger: logger,
	})
}

// FromContext returns a scoped logger.
func FromContext(ctx context.Context) Logger {
	c, ok := ctx.Value(ctxkey{}).(*ctxkey)
	if ok {
		return c.logger
	}
	return nil
}

// Track a field across the lifetime of the context. Unlike GRPC's
// logging.InjectFields, tracked values will persist back up the call-chain to
// the logging interceptor. As such, do not use this as a convenience in loops,
// without creating a new scope using `logging.With(ctx, logger.Named("foo"))`.
func Track(ctx context.Context, field string, value interface{}) {
	c, ok := ctx.Value(ctxkey{}).(*ctxkey)
	if ok {
		c.logger = c.logger.With(field, value)
	}
}

// Logger provides an abstract logging interface designed around uber-go/zap's
// sugared logger, but is intended to provide interop with other libraries.
type Logger interface {
	Debug(args ...interface{})
	Debugw(msg string, keysAndValues ...interface{})
	Debugf(msg string, args ...interface{})
	Info(args ...interface{})
	Infow(msg string, keysAndValues ...interface{})
	Infof(msg string, args ...interface{})
	Warn(args ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Warnf(msg string, args ...interface{})
	Error(args ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
	Errorf(msg string, args ...interface{})
	Panic(args ...interface{})
	Panicw(msg string, keysAndValues ...interface{})
	Panicf(msg string, args ...interface{})
	Fatal(args ...interface{})
	Fatalw(msg string, keysAndValues ...interface{})
	Fatalf(msg string, args ...interface{})

	// Named creates a child logger with the given name.
	Named(name string) Logger

	// With creates a child logger and attaches structured cotnext to it.
	With(field string, value interface{}) Logger
}

func Debug(ctx context.Context, msg string) {
	FromContext(ctx).Debug(msg)
}

func Debugw(ctx context.Context, msg string, fields ...interface{}) {
	FromContext(ctx).Debugw(msg, fields...)
}

func Debugf(ctx context.Context, msg string, args ...interface{}) {
	FromContext(ctx).Debugf(msg, args...)
}

func Info(ctx context.Context, msg string) {
	FromContext(ctx).Info(msg)
}

func Infow(ctx context.Context, msg string, fields ...interface{}) {
	FromContext(ctx).Infow(msg, fields...)
}

func Infof(ctx context.Context, msg string, args ...interface{}) {
	FromContext(ctx).Infof(msg, args...)
}

func Warn(ctx context.Context, msg string) {
	FromContext(ctx).Warn(msg)
}

func Warnw(ctx context.Context, msg string, fields ...interface{}) {
	FromContext(ctx).Warnw(msg, fields...)
}

func Warnf(ctx context.Context, msg string, args ...interface{}) {
	FromContext(ctx).Warnf(msg, args...)
}

func Error(ctx context.Context, msg string) {
	FromContext(ctx).Error(msg)
}

func Errorw(ctx context.Context, msg string, fields ...interface{}) {
	FromContext(ctx).Errorw(msg, fields...)
}

func Errorf(ctx context.Context, msg string, args ...interface{}) {
	FromContext(ctx).Errorf(msg, args...)
}

func Panic(ctx context.Context, msg string) {
	FromContext(ctx).Panic(msg)
}

func Panicw(ctx context.Context, msg string, fields ...interface{}) {
	FromContext(ctx).Panicw(msg, fields...)
}

func Panicf(ctx context.Context, msg string, args ...interface{}) {
	FromContext(ctx).Panicf(msg, args...)
}

func Fatal(ctx context.Context, msg string) {
	FromContext(ctx).Fatal(msg)
}

func Fatalw(ctx context.Context, msg string, fields ...interface{}) {
	FromContext(ctx).Fatalw(msg, fields...)
}

func Fatalf(ctx context.Context, msg string, args ...interface{}) {
	FromContext(ctx).Fatalf(msg, args...)
}
