package test

import (
	"io"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Logger(dest io.Writer) *zap.Logger {
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.AddSync(dest), zap.DebugLevel)
	return zap.New(core)
}
