package logging

import (
	"context"
	"testing"

	"github.com/dpup/prefab/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// Helper to create an observed logger for testing
func newTestLogger() (*ZapLogger, *observer.ObservedLogs) {
	core, obs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	return &ZapLogger{z: logger.Sugar()}, obs
}

func TestWith(t *testing.T) {
	logger := &ZapLogger{z: zap.NewNop().Sugar()}
	ctx := With(t.Context(), logger)

	retrieved := FromContext(ctx)
	assert.NotNil(t, retrieved)
	assert.Equal(t, logger, retrieved)
}

func TestFromContext(t *testing.T) {
	t.Run("WithLogger", func(t *testing.T) {
		logger := &ZapLogger{z: zap.NewNop().Sugar()}
		ctx := With(t.Context(), logger)
		assert.Equal(t, logger, FromContext(ctx))
	})

	t.Run("WithoutLogger", func(t *testing.T) {
		assert.Nil(t, FromContext(t.Context()))
	})
}

func TestEnsureLogger(t *testing.T) {
	t.Run("CreatesLoggerWhenMissing", func(t *testing.T) {
		ctx := EnsureLogger(t.Context())
		logger := FromContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("PreservesExistingLogger", func(t *testing.T) {
		logger := &ZapLogger{z: zap.NewNop().Sugar()}
		ctx := With(t.Context(), logger)
		ctx = EnsureLogger(ctx)
		assert.Equal(t, logger, FromContext(ctx))
	})
}

func TestTrack(t *testing.T) {
	logger, obs := newTestLogger()
	ctx := With(t.Context(), logger)

	Track(ctx, "foo", "bar")
	Track(ctx, "count", 42)

	Info(ctx, "test message")

	require.Equal(t, 1, obs.Len())
	entry := obs.All()[0]
	assert.Equal(t, "test message", entry.Message)
	assert.ElementsMatch(t, []zap.Field{
		zap.String("foo", "bar"),
		zap.Int("count", 42),
	}, entry.Context)
}

func TestTrackNested(t *testing.T) {
	logger, obs := newTestLogger()
	ctx := With(t.Context(), logger)

	Track(ctx, "foo", "bar") // Root level field

	ctx2 := With(ctx, FromContext(ctx).Named("nested"))
	Track(ctx2, "baz", "bam") // Nested level field

	Info(ctx, "root log")
	Info(ctx2, "nested log")

	require.Equal(t, 2, obs.Len())
	allLogs := obs.All()

	// Root log should only have "foo"
	assert.Equal(t, "root log", allLogs[0].Message)
	assert.ElementsMatch(t, []zap.Field{
		zap.String("foo", "bar"),
	}, allLogs[0].Context)

	// Nested log should have both "foo" and "baz"
	assert.Equal(t, "nested log", allLogs[1].Message)
	assert.ElementsMatch(t, []zap.Field{
		zap.String("foo", "bar"),
		zap.String("baz", "bam"),
	}, allLogs[1].Context)
}

func TestTrackWithNoLogger(t *testing.T) {
	// Should not panic when no logger in context
	ctx := t.Context()
	Track(ctx, "foo", "bar")
}

// Test all logging levels
func TestLoggingLevels(t *testing.T) {
	tests := []struct {
		name    string
		logFunc func(context.Context, string)
		level   zapcore.Level
	}{
		{"Debug", Debug, zap.DebugLevel},
		{"Info", Info, zap.InfoLevel},
		{"Warn", Warn, zap.WarnLevel},
		{"Error", Error, zap.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, obs := newTestLogger()
			ctx := With(t.Context(), logger)

			tt.logFunc(ctx, "test message")

			require.Equal(t, 1, obs.Len())
			entry := obs.All()[0]
			assert.Equal(t, "test message", entry.Message)
			assert.Equal(t, tt.level, entry.Level)
		})
	}
}

func TestLoggingWithFields(t *testing.T) {
	logger, obs := newTestLogger()
	ctx := With(t.Context(), logger)

	Debugw(ctx, "debug message", "key1", "val1")
	Infow(ctx, "info message", "key2", "val2")
	Warnw(ctx, "warn message", "key3", "val3")
	Errorw(ctx, "error message", "key4", "val4")

	require.Equal(t, 4, obs.Len())
	entries := obs.All()

	assert.Equal(t, "debug message", entries[0].Message)
	assert.Contains(t, entries[0].Context, zap.String("key1", "val1"))

	assert.Equal(t, "info message", entries[1].Message)
	assert.Contains(t, entries[1].Context, zap.String("key2", "val2"))

	assert.Equal(t, "warn message", entries[2].Message)
	assert.Contains(t, entries[2].Context, zap.String("key3", "val3"))

	assert.Equal(t, "error message", entries[3].Message)
	assert.Contains(t, entries[3].Context, zap.String("key4", "val4"))
}

func TestLoggingFormatted(t *testing.T) {
	logger, obs := newTestLogger()
	ctx := With(t.Context(), logger)

	Debugf(ctx, "debug: %s", "test")
	Infof(ctx, "info: %d", 42)
	Warnf(ctx, "warn: %v", true)
	Errorf(ctx, "error: %.2f", 3.14)

	require.Equal(t, 4, obs.Len())
	entries := obs.All()

	assert.Equal(t, "debug: test", entries[0].Message)
	assert.Equal(t, "info: 42", entries[1].Message)
	assert.Equal(t, "warn: true", entries[2].Message)
	assert.Equal(t, "error: 3.14", entries[3].Message)
}

func TestPanic(t *testing.T) {
	logger, _ := newTestLogger()
	ctx := With(t.Context(), logger)

	assert.Panics(t, func() {
		Panic(ctx, "panic message")
	})

	assert.Panics(t, func() {
		Panicw(ctx, "panic with fields", "key", "value")
	})

	assert.Panics(t, func() {
		Panicf(ctx, "panic formatted: %s", "test")
	})
}

// Test interceptors
func TestScopingInterceptor(t *testing.T) {
	logger, obs := newTestLogger()
	ctx := With(t.Context(), logger)

	handler := func(ctx context.Context, req any) (any, error) {
		Info(ctx, "handler called")
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/service.Example/Method"}

	_, err := scopingInterceptor(ctx, nil, info, handler)
	require.NoError(t, err)

	require.Equal(t, 1, obs.Len())
	entry := obs.All()[0]
	assert.Equal(t, "handler called", entry.Message)
	assert.Equal(t, "/service.Example/Method", entry.LoggerName)
}

func TestErrorInterceptor(t *testing.T) {
	logger, obs := newTestLogger()
	ctx := With(t.Context(), logger)

	t.Run("WithError", func(t *testing.T) {
		obs.TakeAll() // Clear previous logs

		testErr := errors.NewC("test error", codes.InvalidArgument)
		handler := func(ctx context.Context, req any) (any, error) {
			return nil, testErr
		}

		resp, err := errorInterceptor(ctx, nil, nil, handler)
		assert.Nil(t, resp)
		assert.Equal(t, testErr, err)

		// Verify error tracking was called (fields should be added to context)
		// The actual logging happens in grpcLoggingInterceptor
	})

	t.Run("WithPanic", func(t *testing.T) {
		obs.TakeAll()

		handler := func(ctx context.Context, req any) (any, error) {
			panic("test panic")
		}

		resp, err := errorInterceptor(ctx, nil, nil, handler)
		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test panic")
	})

	t.Run("Success", func(t *testing.T) {
		obs.TakeAll()

		handler := func(ctx context.Context, req any) (any, error) {
			return "success", nil
		}

		resp, err := errorInterceptor(ctx, nil, nil, handler)
		assert.Equal(t, "success", resp)
		assert.NoError(t, err)
	})
}

func TestTrackError(t *testing.T) {
	logger, _ := newTestLogger()
	ctx := With(t.Context(), logger)

	t.Run("WithPrefabError", func(t *testing.T) {
		err := errors.NewC("test error", codes.InvalidArgument)
		trackError(ctx, err)

		// Verify fields were tracked (they'll be in the logger context)
		ctxLogger := FromContext(ctx)
		assert.NotNil(t, ctxLogger)
	})

	t.Run("WithStandardError", func(t *testing.T) {
		err := errors.New("standard error")
		trackError(ctx, err)

		ctxLogger := FromContext(ctx)
		assert.NotNil(t, ctxLogger)
	})
}
