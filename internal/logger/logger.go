package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(logFilePath string) (*zap.Logger, error) {
	// only log errors to the file
	warnFile, err := os.OpenFile(logFilePath, os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	encoderConfig := zap.NewDevelopmentConfig()
	encoder := zapcore.NewConsoleEncoder(encoderConfig.EncoderConfig)

	fileCore := zapcore.NewCore(
		encoder,
		zapcore.AddSync(warnFile),
		zapcore.WarnLevel,
	)

	stdoutCore := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)

	core := zapcore.NewTee(fileCore, stdoutCore)

	logger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.WarnLevel),
		zap.Development(),
	)

	return logger, nil
}