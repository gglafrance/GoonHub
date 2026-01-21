package logging

import (
	"goonhub/internal/config"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.Logger
}

func New(cfg *config.Config) (*Logger, error) {
	var zapConfig zap.Config

	if cfg.Environment == "production" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig = getEnhancedEncoderConfig()
	}

	// Set level
	level, err := zapcore.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}
	zapConfig.Level = zap.NewAtomicLevelAt(level)

	// Build logger
	logger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{Logger: logger}, nil
}

// Default returns a basic logger for when config isn't available yet
func Default() *Logger {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig = getEnhancedEncoderConfig()
	logger, _ := config.Build()
	return &Logger{Logger: logger}
}

func getEnhancedEncoderConfig() zapcore.EncoderConfig {
	encConfig := zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    encodeLevel,
		EncodeTime:     encodeTime,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	encConfig.ConsoleSeparator = "  "

	return encConfig
}

func encodeLevel(l zapcore.Level, p zapcore.PrimitiveArrayEncoder) {
	var level string
	var colorCode string

	switch l {
	case zapcore.DebugLevel:
		level = "DEBUG"
		colorCode = "\x1b[1;90m"
	case zapcore.InfoLevel:
		level = "INFO"
		colorCode = "\x1b[1;96m"
	case zapcore.WarnLevel:
		level = "WARN"
		colorCode = "\x1b[1;93m"
	case zapcore.ErrorLevel:
		level = "ERROR"
		colorCode = "\x1b[1;91m"
	case zapcore.FatalLevel:
		level = "FATAL"
		colorCode = "\x1b[1;95m"
	case zapcore.PanicLevel:
		level = "PANIC"
		colorCode = "\x1b[1;95m"
	default:
		level = l.String()
		colorCode = "\x1b[0m"
	}

	buf := buffer.Buffer{}
	buf.AppendString(colorCode)
	buf.AppendString(level)
	buf.AppendString("\x1b[0m")
	p.AppendString(buf.String())
}

func encodeTime(t time.Time, p zapcore.PrimitiveArrayEncoder) {
	p.AppendString("\x1b[35m" + t.Format("15:04:05.000") + "\x1b[0m")
}
