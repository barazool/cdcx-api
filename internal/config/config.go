package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	APIKey    string
	APISecret string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	apiKey := os.Getenv("COINDCX_API_KEY")
	apiSecret := os.Getenv("COINDCX_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("COINDCX_API_KEY and COINDCX_API_SECRET must be set in .env file")
	}

	return &Config{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}, nil
}
