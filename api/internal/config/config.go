package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port string
	Env  string

	DatabaseURL string
	RedisURL    string

	JWTPrivateKey string
	JWTPublicKey  string

	S3Bucket  string
	AWSRegion string

	GoogleClientID     string
	GoogleClientSecret string

	FrontendURL string
}

func Load() (*Config, error) {
	c := &Config{
		Port:               getenv("PORT", "8080"),
		Env:                getenv("ENV", "local"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		RedisURL:           os.Getenv("REDIS_URL"),
		JWTPrivateKey:      os.Getenv("JWT_PRIVATE_KEY"),
		JWTPublicKey:       os.Getenv("JWT_PUBLIC_KEY"),
		S3Bucket:           os.Getenv("S3_BUCKET"),
		AWSRegion:          getenv("AWS_REGION", "ap-southeast-1"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		FrontendURL:        getenv("FRONTEND_URL", "http://localhost:3000"),
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	required := map[string]string{
		"DATABASE_URL":    c.DatabaseURL,
		"REDIS_URL":       c.RedisURL,
		"JWT_PRIVATE_KEY": c.JWTPrivateKey,
		"JWT_PUBLIC_KEY":  c.JWTPublicKey,
	}
	for k, v := range required {
		if v == "" {
			return fmt.Errorf("required env var %s is not set", k)
		}
	}
	if _, err := strconv.Atoi(c.Port); err != nil {
		return fmt.Errorf("PORT must be a number, got %q", c.Port)
	}
	return nil
}

func (c *Config) IsLocal() bool { return c.Env == "local" }

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
