package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func notSetErr(envVar string) error {
	return fmt.Errorf("%q environment variable not set", envVar)
}

type DBConfig struct {
	URL string
}

type Config struct {
	App    *AppConfig
	Server *ServerConfig
	Worker *WorkerConfig
	DB     *DBConfig
}

func Load() (*Config, error) {
	if os.Getenv("APP_ENV") == "dev" {
		if envFile := os.Getenv("ENV_FILE"); envFile != "" {
			err := godotenv.Load(envFile)
			if err != nil {
				return nil, err
			}
		}
	}

	app, err := LoadAppConfig()
	if err != nil {
		return nil, err
	}
	server, err := LoadServerConfig()
	if err != nil {
		return nil, err
	}
	worker, err := LoadWorkerConfig()
	if err != nil {
		return nil, err
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, notSetErr("DATABASE_URL")
	}
	return &Config{App: app, Server: server, Worker: worker, DB: &DBConfig{URL: dbURL}}, nil
}
