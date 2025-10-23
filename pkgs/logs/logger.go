package logs

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	stdcfg = zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "lvl",
		NameKey:        "mod",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	logger *zap.SugaredLogger
)

type contextKey string

const (
	contextKeyLogger contextKey = "yg-go-logger"
)

func init() {
	core := zapcore.NewTee(zapcore.NewCore(
		zapcore.NewJSONEncoder(stdcfg),
		zapcore.Lock(os.Stdout),
		zap.NewAtomicLevelAt(zap.InfoLevel)))
	logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
}

// DebugContextf 调试
func DebugContextf(ctx context.Context, template string, args ...interface{}) {
	LoggerFromContext(ctx).Debugf(template, args...)
}

// InfoContextf 提示信息
func InfoContextf(ctx context.Context, template string, args ...interface{}) {
	LoggerFromContext(ctx).Infof(template, args...)
}

// WarnContextf 警告信息
func WarnContextf(ctx context.Context, template string, args ...interface{}) {
	LoggerFromContext(ctx).Warnf(template, args...)
}

// ErrorContextf 错误信息
func ErrorContextf(ctx context.Context, template string, args ...interface{}) {
	LoggerFromContext(ctx).Errorf(template, args...)
}

// FatalContextf 致命错误
func FatalContextf(ctx context.Context, template string, args ...interface{}) {
	LoggerFromContext(ctx).Fatalf(template, args...)
}

func DebugContextw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	LoggerFromContext(ctx).Debugw(msg, keysAndValues...)
}

func InfoContextw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	LoggerFromContext(ctx).Infow(msg, keysAndValues...)
}

func WarnContextw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	LoggerFromContext(ctx).Warnw(msg, keysAndValues...)
}

func ErrorContextw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	LoggerFromContext(ctx).Errorw(msg, keysAndValues...)
}

func FatalContextw(ctx context.Context, msg string, keysAndValues ...interface{}) {
	LoggerFromContext(ctx).Fatalw(msg, keysAndValues...)
}

// WithContextFields 设置日志字段上下文
func WithContextFields(ctx context.Context, fields ...interface{}) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	l := logger
	val := ctx.Value(contextKeyLogger)
	if val != nil {
		var ok bool
		l, ok = val.(*zap.SugaredLogger)
		if !ok {
			l = logger
		}
	}
	l = l.With(fields...)
	return context.WithValue(ctx, contextKeyLogger, l)
}

// WithContextLogger 设置日志上下文
func WithContextLogger(ctx context.Context, l *zap.SugaredLogger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKeyLogger, l)
}

// SetContextFields 设置日志上下文
func SetContextFields(ctx context.Context, fields ...interface{}) {
	ctx = WithContextFields(ctx, fields...)
}

// SetContextLogger 设置日志上下文
func SetContextLogger(ctx context.Context, l *zap.SugaredLogger) {
	ctx = WithContextLogger(ctx, l)
}

// LoggerFromContext 获取日志上下文
func LoggerFromContext(ctx context.Context) *zap.SugaredLogger {
	val := ctx.Value(contextKeyLogger)
	if val == nil {
		return logger
	}
	l, ok := val.(*zap.SugaredLogger)
	if !ok {
		return logger
	}
	return l
}

// SetLevel 设置默认日志级别
func SetLevel(lvl zapcore.Level) {
	core := zapcore.NewTee(zapcore.NewCore(
		zapcore.NewJSONEncoder(stdcfg),
		zapcore.Lock(os.Stdout),
		zap.NewAtomicLevelAt(lvl)))
	logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
}
