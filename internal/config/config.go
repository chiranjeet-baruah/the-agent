// Package config loads runtime configuration from config/config.yaml via
// viper, with environment variables of the same name overriding file values.
package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/viper"
)

// Candidate is one provider worth trying at startup. ModelPreferences is the
// config.yaml order to match against that provider's live model list;
// ModelOverride, when non-empty (only possible via explicit provider
// selection), is used as-is instead, unvalidated.
type Candidate struct {
	Provider         string
	ModelPreferences []string
	ModelOverride    string
	BaseURL          string
	APIKey           string
}

// providerConfig is one entry under the config.yaml "providers" map.
type providerConfig struct {
	Models    []string `mapstructure:"model"`
	BaseURL   string   `mapstructure:"base_url"`
	APIKeyEnv string   `mapstructure:"api_key_env"`
}

// Load returns the providers worth trying, in order, plus the HTTP port.
// Load itself never resolves a final model name — main.go does that against
// each candidate's live model list (see ModelPreferences/ModelOverride).
//
// If the "provider"/PROVIDER key is set, only that provider is returned (and
// "model"/MODEL, if set, becomes its ModelOverride) — Load fails fast if
// that provider is unknown or its API key env var is unset.
//
// Otherwise every provider whose api_key_env is actually set in the
// environment is returned, in config.yaml order, for main.go to probe in
// turn; Load fails only if none of them have a key set.
func Load() ([]Candidate, string, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("config")

	v.SetDefault("port", "8080")
	v.AutomaticEnv()

	var notFound viper.ConfigFileNotFoundError
	if err := v.ReadInConfig(); err != nil && !errors.As(err, &notFound) {
		return nil, "", fmt.Errorf("config: read config/config.yaml: %w", err)
	}

	all := knownProviders(v)
	if len(all) == 0 {
		return nil, "", errors.New("config: no providers configured — config/config.yaml must define a providers map")
	}
	port := v.GetString("port")

	if explicit := v.GetString("provider"); explicit != "" {
		if !slices.Contains(all, explicit) {
			return nil, "", fmt.Errorf("config: unknown provider %q (known: %s)", explicit, strings.Join(all, ", "))
		}
		pc, err := loadProvider(v, explicit)
		if err != nil {
			return nil, "", err
		}
		apiKey := os.Getenv(pc.APIKeyEnv)
		if apiKey == "" {
			return nil, "", fmt.Errorf("config: %s is required (set it in the environment for provider %q)", pc.APIKeyEnv, explicit)
		}
		c := Candidate{Provider: explicit, ModelPreferences: pc.Models, ModelOverride: v.GetString("model"), BaseURL: pc.BaseURL, APIKey: apiKey}
		return []Candidate{c}, port, nil
	}

	var candidates []Candidate
	var envVars []string
	for _, name := range all {
		pc, err := loadProvider(v, name)
		if err != nil {
			return nil, "", err
		}
		envVars = append(envVars, pc.APIKeyEnv)
		apiKey := os.Getenv(pc.APIKeyEnv)
		if apiKey == "" {
			continue
		}
		candidates = append(candidates, Candidate{Provider: name, ModelPreferences: pc.Models, BaseURL: pc.BaseURL, APIKey: apiKey})
	}
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("config: no provider has its API key set (checked: %s)", strings.Join(envVars, ", "))
	}
	return candidates, port, nil
}

func loadProvider(v *viper.Viper, name string) (providerConfig, error) {
	key := "providers." + name
	var pc providerConfig
	if err := v.UnmarshalKey(key, &pc); err != nil {
		return pc, fmt.Errorf("config: parse %s: %w", key, err)
	}
	if len(pc.Models) == 0 {
		return pc, fmt.Errorf("config: %s.model must list at least one model", key)
	}
	return pc, nil
}

func knownProviders(v *viper.Viper) []string {
	names := make([]string, 0, len(v.GetStringMap("providers")))
	for name := range v.GetStringMap("providers") {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
