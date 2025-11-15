package config

import "os"

type Config struct {
	ReadOnly bool
}

func NewConfig() *Config {
	isReadOnlyMode := os.Getenv("READONLY_MODE") == "true"

	return &Config{
		ReadOnly: isReadOnlyMode,
	}
}
