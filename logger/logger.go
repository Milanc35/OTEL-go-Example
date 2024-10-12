package logger

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var LogHandle ApplicationLogger
var once sync.Once

type ApplicationLogger struct {
	logger *zap.Logger
}

func InitLogger(lvl, serviceName, environment string) error {
	level, err := parseLevel(lvl)
	LogHandle = getLogger(level, serviceName, environment)
	return err
}

func getLogger(level zapcore.Level, serviceName string, environment string) ApplicationLogger {
	once.Do(func() {
		encoderConfig := ecszap.EncoderConfig{
			EncodeName:     zap.NewProductionEncoderConfig().EncodeName,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   ecszap.FullCallerEncoder,
		}
		core := ecszap.NewCore(encoderConfig, os.Stdout, level)
		l := zap.New(core, zap.AddCaller())
		l = l.With(zap.String("app", serviceName)).With(zap.String("env", environment))

		LogHandle = ApplicationLogger{
			logger: l,
		}
	})

	return LogHandle
}

type ContextKey string

var contextKeys = map[ContextKey]string{
	ContextKey("trace_id"): "traceID",
	ContextKey("span_id"):  "spanID",
	ContextKey("user_id"):  "userID",
}

func (l *ApplicationLogger) Debugf(msg string, args ...zapcore.Field) {
	l.logger.Debug(msg, args...)
}
func (l *ApplicationLogger) Infof(msg string, args ...zapcore.Field) {
	l.logger.Info(msg, args...)
}
func (l *ApplicationLogger) Errorf(msg string, args ...zapcore.Field) {
	l.logger.Error(msg, args...)
}
func (l *ApplicationLogger) Fatalf(msg string, args ...zapcore.Field) {
	l.logger.Fatal(msg, args...)
}
func (l *ApplicationLogger) Debug(msg string) {
	l.logger.Debug(msg)
}
func (l *ApplicationLogger) Info(msg string) {
	l.logger.Info(msg)
}
func (l *ApplicationLogger) Error(msg string) {
	l.logger.Error(msg)
}
func (l *ApplicationLogger) Fatal(err error) {
	l.logger.Fatal(err.Error())
}

func (l *ApplicationLogger) With(fields ...zapcore.Field) *zap.Logger {
	return l.logger.With(fields...)
}

func (l *ApplicationLogger) WithContext(ctx context.Context) *zap.Logger {
	logFields := []zapcore.Field{}
	for key, val := range contextKeys {
		logFields = append(logFields, zap.String(string(key), val))
	}

	return l.logger.With(logFields...)
}

func parseLevel(lvl string) (zapcore.Level, error) {
	switch strings.ToLower(lvl) {
	case "debug":
		return zap.DebugLevel, nil
	case "info":
		return zap.InfoLevel, nil
	case "warn":
		return zap.WarnLevel, nil
	case "error":
		return zap.ErrorLevel, nil
	}

	return zap.InfoLevel, fmt.Errorf("invalid log level <%v>", lvl)
}
