package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(level zapcore.Level, encoding Encoding) (*zap.Logger, error) {
	if !SupportedEncodings.Contains(encoding) {
		return nil, InvalidEncodingError{encoding: encoding}
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "time"
	encCfg.MessageKey = "message"
	encCfg.EncodeTime = zapcore.EpochNanosTimeEncoder

	loggerCfg := zap.NewProductionConfig()
	loggerCfg.Level = zap.NewAtomicLevelAt(level)
	loggerCfg.EncoderConfig = encCfg
	loggerCfg.Encoding = encoding.String()

	log, err := loggerCfg.Build(zap.AddCaller())
	if err != nil {
		return nil, err
	}

	return log, nil
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
