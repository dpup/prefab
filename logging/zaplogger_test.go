package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewDevLogger(t *testing.T) {
	logger := NewDevLogger()
	require.NotNil(t, logger)
	assert.IsType(t, &ZapLogger{}, logger)
}

func TestNewProdLogger(t *testing.T) {
	logger := NewProdLogger()
	require.NotNil(t, logger)
	assert.IsType(t, &ZapLogger{}, logger)
}

func TestZapLoggerDebug(t *testing.T) {
	core, obs := observer.New(zap.DebugLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Debug("debug message")
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "debug message", obs.All()[0].Message)
	assert.Equal(t, zap.DebugLevel, obs.All()[0].Level)
}

func TestZapLoggerDebugw(t *testing.T) {
	core, obs := observer.New(zap.DebugLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Debugw("debug message", "key", "value")
	require.Equal(t, 1, obs.Len())
	entry := obs.All()[0]
	assert.Equal(t, "debug message", entry.Message)
	assert.Contains(t, entry.Context, zap.String("key", "value"))
}

func TestZapLoggerDebugf(t *testing.T) {
	core, obs := observer.New(zap.DebugLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Debugf("debug: %s %d", "test", 42)
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "debug: test 42", obs.All()[0].Message)
}

func TestZapLoggerInfo(t *testing.T) {
	core, obs := observer.New(zap.InfoLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Info("info message")
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "info message", obs.All()[0].Message)
	assert.Equal(t, zap.InfoLevel, obs.All()[0].Level)
}

func TestZapLoggerInfow(t *testing.T) {
	core, obs := observer.New(zap.InfoLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Infow("info message", "key", "value")
	require.Equal(t, 1, obs.Len())
	entry := obs.All()[0]
	assert.Equal(t, "info message", entry.Message)
	assert.Contains(t, entry.Context, zap.String("key", "value"))
}

func TestZapLoggerInfof(t *testing.T) {
	core, obs := observer.New(zap.InfoLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Infof("info: %s", "test")
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "info: test", obs.All()[0].Message)
}

func TestZapLoggerWarn(t *testing.T) {
	core, obs := observer.New(zap.WarnLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Warn("warn message")
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "warn message", obs.All()[0].Message)
	assert.Equal(t, zap.WarnLevel, obs.All()[0].Level)
}

func TestZapLoggerWarnw(t *testing.T) {
	core, obs := observer.New(zap.WarnLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Warnw("warn message", "key", "value")
	require.Equal(t, 1, obs.Len())
	entry := obs.All()[0]
	assert.Equal(t, "warn message", entry.Message)
	assert.Contains(t, entry.Context, zap.String("key", "value"))
}

func TestZapLoggerWarnf(t *testing.T) {
	core, obs := observer.New(zap.WarnLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Warnf("warn: %s", "test")
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "warn: test", obs.All()[0].Message)
}

func TestZapLoggerError(t *testing.T) {
	core, obs := observer.New(zap.ErrorLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Error("error message")
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "error message", obs.All()[0].Message)
	assert.Equal(t, zap.ErrorLevel, obs.All()[0].Level)
}

func TestZapLoggerErrorw(t *testing.T) {
	core, obs := observer.New(zap.ErrorLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Errorw("error message", "key", "value")
	require.Equal(t, 1, obs.Len())
	entry := obs.All()[0]
	assert.Equal(t, "error message", entry.Message)
	assert.Contains(t, entry.Context, zap.String("key", "value"))
}

func TestZapLoggerErrorf(t *testing.T) {
	core, obs := observer.New(zap.ErrorLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	logger.Errorf("error: %s", "test")
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "error: test", obs.All()[0].Message)
}

func TestZapLoggerPanic(t *testing.T) {
	core, obs := observer.New(zapcore.PanicLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	assert.Panics(t, func() {
		logger.Panic("panic message")
	})
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "panic message", obs.All()[0].Message)
}

func TestZapLoggerPanicw(t *testing.T) {
	core, obs := observer.New(zapcore.PanicLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	assert.Panics(t, func() {
		logger.Panicw("panic message", "key", "value")
	})
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "panic message", obs.All()[0].Message)
}

func TestZapLoggerPanicf(t *testing.T) {
	core, obs := observer.New(zapcore.PanicLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	assert.Panics(t, func() {
		logger.Panicf("panic: %s", "test")
	})
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "panic: test", obs.All()[0].Message)
}

// Fatal tests are skipped as they call os.Exit and are hard to test
// We trust that zap's Fatal implementation works correctly

func TestZapLoggerNamed(t *testing.T) {
	core, obs := observer.New(zap.InfoLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	named := logger.Named("test")
	require.NotNil(t, named)
	require.IsType(t, &ZapLogger{}, named)

	named.Info("test message")
	require.Equal(t, 1, obs.Len())
	assert.Equal(t, "test", obs.All()[0].LoggerName)
}

func TestZapLoggerWith(t *testing.T) {
	core, obs := observer.New(zap.InfoLevel)
	logger := &ZapLogger{z: zap.New(core).Sugar()}

	withFields := logger.With("key", "value")
	require.NotNil(t, withFields)
	require.IsType(t, &ZapLogger{}, withFields)

	withFields.Info("test message")
	require.Equal(t, 1, obs.Len())
	entry := obs.All()[0]
	assert.Equal(t, "test message", entry.Message)
	assert.Contains(t, entry.Context, zap.String("key", "value"))
}
