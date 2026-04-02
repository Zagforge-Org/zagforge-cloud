package config

import (
	"fmt"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	App    AppConfig    `envPrefix:""`
	Server ServerConfig `envPrefix:""`
	DB     DBConfig     `envPrefix:""`
	Redis  RedisConfig  `envPrefix:""`
	CORS   CORSConfig   `envPrefix:""`
}

type ServerConfig struct {
	Port string `env:"PORT,required"`
}

type DBConfig struct {
	URL string `env:"AUTH_DATABASE_URL,required"`
}

type RedisConfig struct {
	URL string `env:"REDIS_URL,required"`
}

type CORSConfig struct {
	AllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:","`
}

func Load() (*Config, error) {
	if os.Getenv("APP_ENV") == "dev" {
		if envFile := os.Getenv("ENV_FILE"); envFile != "" {
			if err := godotenv.Load(envFile); err != nil {
				return nil, err
			}
		}
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}
