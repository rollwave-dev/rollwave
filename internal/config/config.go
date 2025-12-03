// internal/config/config.go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Project string `yaml:"project"`

	Stack struct {
		Name        string `yaml:"name"`
		ComposeFile string `yaml:"compose_file"`
	} `yaml:"stack"`

	Secrets struct {
		StackPrefix string `yaml:"stack_prefix"`
	} `yaml:"secrets"`

	Deploy struct {
		WithSecrets bool `yaml:"with_secrets"`
	} `yaml:"deploy"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
