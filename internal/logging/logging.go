package logging

import (
	"context"
	"io"
	"log/slog"

	"github.com/sirupsen/logrus"
)

// Configure routes standard-library and Compose SDK logs through the app handler.
// Call during startup, before starting concurrent application work.
func Configure(logger *slog.Logger) {
	slog.SetDefault(logger)
	compose := logrus.StandardLogger()
	compose.ReplaceHooks(make(logrus.LevelHooks))
	compose.AddHook(composeHook{logger: logger})
	compose.SetOutput(io.Discard)
	// Do not enable SDK debug/trace dumps of resolved Compose configuration.
	compose.SetLevel(logrus.InfoLevel)
}

type composeHook struct {
	logger *slog.Logger
}

func (composeHook) Levels() []logrus.Level { return logrus.AllLevels }

func (hook composeHook) Fire(entry *logrus.Entry) error {
	level := slog.LevelInfo
	switch entry.Level {
	case logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel:
		level = slog.LevelError
	case logrus.WarnLevel:
		level = slog.LevelWarn
	case logrus.DebugLevel, logrus.TraceLevel:
		level = slog.LevelDebug
	}
	ctx := entry.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if !hook.logger.Enabled(ctx, level) {
		return nil
	}
	fields := make([]slog.Attr, 0, len(entry.Data))
	for key, value := range entry.Data {
		fields = append(fields, slog.Any(key, value))
	}
	attrs := []slog.Attr{slog.String("source", "compose")}
	if len(fields) != 0 {
		attrs = append(attrs, slog.GroupAttrs("fields", fields...))
	}
	hook.logger.LogAttrs(ctx, level, entry.Message, attrs...)
	return nil
}
