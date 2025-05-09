package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestTrack(t *testing.T) {
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	ctx := With(t.Context(), &ZapLogger{z: observedLogger.Sugar()})
	Track(ctx, "foo", "bar") // Should be passed on to child logger.

	ctx2 := With(ctx, FromContext(ctx).Named("nested"))
	Track(ctx2, "baz", "bam") // Should not propagate to root logger.

	Info(ctx, "root log")
	Info(ctx2, "nested log")

	if len(observedLogs.All()) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(observedLogs.All()))
	}

	require.Equal(t, 2, observedLogs.Len())
	allLogs := observedLogs.All()
	assert.Equal(t, "root log", allLogs[0].Message)
	assert.ElementsMatch(t, []zap.Field{
		zap.String("foo", "bar"),
	}, allLogs[0].Context)

	assert.Equal(t, "nested log", allLogs[1].Message)
	assert.ElementsMatch(t, []zap.Field{
		zap.String("foo", "bar"),
		zap.String("baz", "bam"),
	}, allLogs[1].Context)
}
