// Package config loads runtime configuration from config/config.yaml via
// viper, with environment variables of the same name overriding file values.
package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	ModelName string
	BaseURL   string
	Port      string
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("config")

	v.SetDefault("model", "ai/llama3.2")
	v.SetDefault("llm_base_url", "http://localhost:12434/engines/v1")
	v.SetDefault("port", "8080")
	v.AutomaticEnv()

	var notFound viper.ConfigFileNotFoundError
	if err := v.ReadInConfig(); err != nil && !errors.As(err, &notFound) {
		return nil, fmt.Errorf("config: read config/config.yaml: %w", err)
	}

	return &Config{
		ModelName: v.GetString("model"),
		BaseURL:   v.GetString("llm_base_url"),
		Port:      v.GetString("port"),
	}, nil
}
