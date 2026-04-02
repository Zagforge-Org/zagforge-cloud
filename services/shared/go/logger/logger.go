package logger

import (
	"go.uber.org/zap"
)

// New creates a zap.Logger configured for the given environment.
// "dev" uses a human-readable console encoder; anything else uses JSON.
func New(env string) (*zap.Logger, error) {
	if env == "dev" {
		return zap.NewDevelopment()
	}
	return zap.NewProduction()
}
