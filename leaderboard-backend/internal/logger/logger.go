package logger

import (
	"go.uber.org/zap"
)

func New(env string) *zap.Logger {
	var logger *zap.Logger
	var err error

	if env == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}

	if err != nil {
		panic(err)
	}

	return logger
}
