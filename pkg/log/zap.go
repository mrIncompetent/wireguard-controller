package log

import (
	"errors"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	FormatJSON    = "json"
	FormatConsole = "console"
)

func New(level *zapcore.Level, format string) (*zap.Logger, error) {
	if format != FormatJSON && format != FormatConsole {
		return nil, errors.New("invalid format specified")
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "time"
	encCfg.MessageKey = "message"
	encCfg.EncodeTime = zapcore.EpochNanosTimeEncoder

	loggerCfg := zap.NewProductionConfig()
	loggerCfg.Level = zap.NewAtomicLevelAt(*level)
	loggerCfg.EncoderConfig = encCfg
	loggerCfg.Encoding = format

	coreLogger, err := loggerCfg.Build(zap.AddCaller())
	if err != nil {
		return nil, err
	}

	return coreLogger, nil
}

func NewTestLog(ws zapcore.WriteSyncer) *zap.Logger {
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), ws, zap.DebugLevel)
	return zap.New(core)
}
