package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// --- Shared Structures ---

type StackConfig struct {
	Name        string `yaml:"name"`
	ComposeFile string `yaml:"compose_file"`
}

type SecretsConfig struct {
	StackPrefix string `yaml:"stack_prefix"`
}

type DeployConfig struct {
	WithSecrets bool `yaml:"with_secrets"`
}

// --- Main Config Structure ---

type Config struct {
	Project string `yaml:"project"`

	Stack   StackConfig   `yaml:"stack"`
	Secrets SecretsConfig `yaml:"secrets"`
	Deploy  DeployConfig  `yaml:"deploy"`

	// Variables injected into environment for envsubst in docker-compose
	Variables map[string]string `yaml:"variables"`

	Environments map[string]Environment `yaml:"environments"`
}

type Environment struct {
	Stack struct {
		Name        string `yaml:"name"`
		ComposeFile string `yaml:"compose_file"`
	} `yaml:"stack"`

	Secrets struct {
		StackPrefix string `yaml:"stack_prefix"`
	} `yaml:"secrets"`

	Deploy struct {
		WithSecrets *bool `yaml:"with_secrets"`
	} `yaml:"deploy"`

	// Variables specific to this environment
	Variables map[string]string `yaml:"variables"`
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
	// Initialize map if nil to avoid panic on assignment later
	if cfg.Variables == nil {
		cfg.Variables = make(map[string]string)
	}
	return &cfg, nil
}

func (c *Config) MergeWithEnv(envName string) (*Config, error) {
	if envName == "" {
		return c, nil
	}

	env, ok := c.Environments[envName]
	if !ok {
		return nil, fmt.Errorf("environment '%s' not defined in config", envName)
	}

	merged := *c

	// 1. Stack Overrides
	if env.Stack.Name != "" {
		merged.Stack.Name = env.Stack.Name
	}
	if env.Stack.ComposeFile != "" {
		merged.Stack.ComposeFile = env.Stack.ComposeFile
	}

	// 2. Secrets Overrides
	if env.Secrets.StackPrefix != "" {
		merged.Secrets.StackPrefix = env.Secrets.StackPrefix
	}

	// 3. Deploy Overrides
	if env.Deploy.WithSecrets != nil {
		merged.Deploy.WithSecrets = *env.Deploy.WithSecrets
	}

	// 4. Variables Merge (NOVÉ)
	// We start with defaults (already in merged.Variables) and overwrite with env specifics
	for k, v := range env.Variables {
		merged.Variables[k] = v
	}

	merged.Environments = nil

	return &merged, nil
}
