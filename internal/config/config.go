package config

import (
	"log"
	"os"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	DbUrl		string	`env:"DB_URL"`
	BotToken	string	`env:"BOT_TOKEN"`
	ServerAddr	string	`env:"SERVER_ADDR"`
	ApiKey		string	`env:"API_KEY"`
	ModelName	string	`env:"MODEL_NAME"`
}

func New() *Config {
	var cfg Config

	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("Failed to initialize config: %v\n", err)
		os.Exit(1)
	}

	return &cfg
}
