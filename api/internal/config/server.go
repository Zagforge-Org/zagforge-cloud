package config

import (
	"fmt"
	"os"
	"strconv"
)

type ServerConfig struct {
	Port string
}

func LoadServerConfig() (*ServerConfig, error) {
	portStr := os.Getenv("PORT")

	if portStr == "" {
		return nil, notSetErr("PORT")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	return &ServerConfig{Port: strconv.Itoa(port)}, nil
}
