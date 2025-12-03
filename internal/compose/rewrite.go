package compose

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// BuildConfig holds information required for the build process.
type BuildConfig struct {
	ServiceName string
	ImageName   string
	Context     string
	Dockerfile  string
}

// ExtractBuildConfigs iterates through services and finds those with a 'build' section.
func ExtractBuildConfigs(yamlBytes []byte) ([]BuildConfig, error) {
	var data struct {
		Services map[string]interface{} `yaml:"services"`
	}
	if err := yaml.Unmarshal(yamlBytes, &data); err != nil {
		return nil, err
	}

	var configs []BuildConfig

	for name, svcBody := range data.Services {
		svc, ok := svcBody.(map[string]interface{})
		if !ok {
			continue
		}

		// Does the service have a build section?
		buildSection, hasBuild := svc["build"]
		if !hasBuild {
			continue
		}

		// Does the service have an image name defined? (Required for push)
		imageName, hasImage := svc["image"].(string)
		if !hasImage || imageName == "" {
			return nil, fmt.Errorf("service '%s' has build section but missing 'image' name (required for push)", name)
		}

		cfg := BuildConfig{
			ServiceName: name,
			ImageName:   imageName,
			Dockerfile:  "Dockerfile", // default
			Context:     ".",          // default
		}

		// Parsing build section (can be string or object)
		switch b := buildSection.(type) {
		case string:
			cfg.Context = b
		case map[string]interface{}:
			if ctx, ok := b["context"].(string); ok {
				cfg.Context = ctx
			}
			if df, ok := b["dockerfile"].(string); ok {
				cfg.Dockerfile = df
			}
		}

		configs = append(configs, cfg)
	}

	return configs, nil
}

// RewriteSecrets modifies the Compose YAML to point to specific secret versions.
func RewriteSecrets(originalYaml []byte, secretMap map[string]string) ([]byte, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal(originalYaml, &data); err != nil {
		return nil, fmt.Errorf("parse compose: %w", err)
	}
	secretsSection, ok := data["secrets"].(map[string]interface{})
	if !ok {
		return originalYaml, nil
	}

	for logicalName, physicalName := range secretMap {
		if secretDef, exists := secretsSection[logicalName]; exists {
			if secretProps, ok := secretDef.(map[string]interface{}); ok {
				secretProps["name"] = physicalName
				secretProps["external"] = true
				secretsSection[logicalName] = secretProps
			}
		}
	}
	data["secrets"] = secretsSection
	return yaml.Marshal(data)
}

// ReplaceImages updates the image tag for specific services and removes the build section.
// newImages map key is serviceName, value is "registry/image:tag".
func ReplaceImages(yamlBytes []byte, newImages map[string]string) ([]byte, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal(yamlBytes, &data); err != nil {
		return nil, err
	}

	services, ok := data["services"].(map[string]interface{})
	if !ok {
		return yamlBytes, nil
	}

	for svcName, newTag := range newImages {
		if svc, ok := services[svcName].(map[string]interface{}); ok {
			svc["image"] = newTag

			// Important: For Swarm deployment, remove the 'build' section
			// to prevent Swarm from attempting to build (which it cannot do natively).
			delete(svc, "build")

			services[svcName] = svc
		}
	}

	data["services"] = services
	return yaml.Marshal(data)
}
