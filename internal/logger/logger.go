package logger

import (
	"log"
	"os"

	"go.uber.org/zap"
)

func New() *zap.Logger {
	logger, err := zap.NewDevelopment()

	if err != nil {
		log.Fatalf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	return logger
}
