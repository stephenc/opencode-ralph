package ralph

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds project configuration.
type Config struct {
	PromptFile      string `json:"prompt_file"`
	ConventionsFile string `json:"conventions_file"`
	SpecsFile       string `json:"specs_file"`
	MaxIterations   int    `json:"max_iterations"`
	MaxPerHour      int    `json:"max_per_hour"`
	MaxPerDay       int    `json:"max_per_day"`
	Model           string `json:"model,omitempty"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		PromptFile:      "PROMPT.md",
		ConventionsFile: "CONVENTIONS.md",
		SpecsFile:       "SPECS.md",
		MaxIterations:   50,
		MaxPerHour:      0,
		MaxPerDay:       0,
	}
}

// LoadConfig loads .ralph/config.json if present.
func LoadConfig() Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(configFile)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

// SaveConfig persists cfg to .ralph/config.json.
func SaveConfig(cfg Config) error {
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		return fmt.Errorf("creating %s directory: %w", ralphDir, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", configFile, err)
	}
	return nil
}

// ConfigView renders the current config as JSON.
func ConfigView() (string, error) {
	cfg := LoadConfig()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling config: %w", err)
	}
	return string(data), nil
}

// ConfigReset resets config to defaults.
func ConfigReset() error {
	cfg := DefaultConfig()
	return SaveConfig(cfg)
}

// ConfigSet updates a single config key.
func ConfigSet(key, value string) error {
	cfg := LoadConfig()

	switch key {
	case "prompt_file":
		cfg.PromptFile = value
	case "conventions_file":
		cfg.ConventionsFile = value
	case "specs_file":
		cfg.SpecsFile = value
	case "max_iterations":
		v, err := parseInt(value)
		if err != nil {
			return fmt.Errorf("parsing max_iterations: %w", err)
		}
		cfg.MaxIterations = v
	case "max_per_hour":
		v, err := parseInt(value)
		if err != nil {
			return fmt.Errorf("parsing max_per_hour: %w", err)
		}
		cfg.MaxPerHour = v
	case "max_per_day":
		v, err := parseInt(value)
		if err != nil {
			return fmt.Errorf("parsing max_per_day: %w", err)
		}
		cfg.MaxPerDay = v
	case "model":
		cfg.Model = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return SaveConfig(cfg)
}

func parseInt(value string) (int, error) {
	var v int
	if _, err := fmt.Sscanf(value, "%d", &v); err != nil {
		return 0, err
	}
	return v, nil
}
