package logging

import "go.uber.org/zap"

// NewDevLogger returns a zap logger that prints dev friendly output.
func NewDevLogger() Logger {
	l, _ := zap.NewDevelopment(zap.AddCallerSkip(2))
	return &ZapLogger{z: l.Sugar()}
}

// NewProdLogger returns a zap logger that outputs JSON.
func NewProdLogger() Logger {
	l, _ := zap.NewProduction(zap.AddCallerSkip(2))
	return &ZapLogger{z: l.Sugar()}
}

// ZapLogger is a logging adapter for a Zap Sugarded Logger.
type ZapLogger struct {
	z *zap.SugaredLogger
}

func (z *ZapLogger) Debug(args ...interface{}) {
	z.z.Debug(args...)
}

func (z *ZapLogger) Debugw(msg string, keysAndValues ...interface{}) {
	z.z.Debugw(msg, keysAndValues...)
}

func (z *ZapLogger) Debugf(msg string, args ...interface{}) {
	z.z.Debugf(msg, args...)
}

func (z *ZapLogger) Info(args ...interface{}) {
	z.z.Info(args...)
}

func (z *ZapLogger) Infow(msg string, keysAndValues ...interface{}) {
	z.z.Infow(msg, keysAndValues...)
}

func (z *ZapLogger) Infof(msg string, args ...interface{}) {
	z.z.Infof(msg, args...)
}

func (z *ZapLogger) Warn(args ...interface{}) {
	z.z.Warn(args...)
}

func (z *ZapLogger) Warnw(msg string, keysAndValues ...interface{}) {
	z.z.Warnw(msg, keysAndValues...)
}

func (z *ZapLogger) Warnf(msg string, args ...interface{}) {
	z.z.Warnf(msg, args...)
}

func (z *ZapLogger) Error(args ...interface{}) {
	z.z.Error(args...)
}

func (z *ZapLogger) Errorw(msg string, keysAndValues ...interface{}) {
	z.z.Errorw(msg, keysAndValues...)
}

func (z *ZapLogger) Errorf(msg string, args ...interface{}) {
	z.z.Errorf(msg, args...)
}

func (z *ZapLogger) Panic(args ...interface{}) {
	z.z.Panic(args...)
}

func (z *ZapLogger) Panicw(msg string, keysAndValues ...interface{}) {
	z.z.Panicw(msg, keysAndValues...)
}

func (z *ZapLogger) Panicf(msg string, args ...interface{}) {
	z.z.Panicf(msg, args...)
}

func (z *ZapLogger) Fatal(args ...interface{}) {
	z.z.Fatal(args...)
}

func (z *ZapLogger) Fatalw(msg string, keysAndValues ...interface{}) {
	z.z.Fatalw(msg, keysAndValues...)
}

func (z *ZapLogger) Fatalf(msg string, args ...interface{}) {
	z.z.Fatalf(msg, args...)
}

func (z *ZapLogger) Named(name string) Logger {
	return &ZapLogger{z: z.z.Named(name)}
}

func (z *ZapLogger) With(field string, value interface{}) Logger {
	return &ZapLogger{z: z.z.With(field, value)}
}
