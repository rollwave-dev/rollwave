package prune

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Run identifies and removes unused secrets for the given stack.
func Run(ctx context.Context, stackName string, stdout, stderr io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	fmt.Fprintf(stdout, "🧹 Pruning secrets for stack '%s'...\n", stackName)

	// 1. List all secrets belonging to this stack
	secrets, err := listSecrets(ctx, stackName)
	if err != nil {
		return err
	}

	// 2. Get list of secrets currently used by services
	usedSecretIDs, err := getUsedSecretIDs(ctx, stackName)
	if err != nil {
		return err
	}

	// 3. Compare and delete
	deletedCount := 0
	for _, s := range secrets {
		// If secret ID is not in the used list -> DELETE
		if !usedSecretIDs[s.ID] {
			fmt.Fprintf(stdout, "   Deleting unused secret: %s\n", s.Name)
			if err := removeSecret(ctx, s.ID); err != nil {
				fmt.Fprintf(stderr, "   ⚠️ Failed to remove %s: %v\n", s.Name, err)
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount == 0 {
		fmt.Fprintln(stdout, "✨ No unused secrets found. Clean.")
	} else {
		fmt.Fprintf(stdout, "🗑️  Deleted %d secrets.\n", deletedCount)
	}

	return nil
}

// --- Helpers ---

type SecretInfo struct {
	ID   string
	Name string
}

func listSecrets(ctx context.Context, stackPrefix string) ([]SecretInfo, error) {
	// docker secret ls --format json
	cmd := exec.CommandContext(ctx, "docker", "secret", "ls", "--format", "{{json .}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}

	var secrets []SecretInfo
	// Docker sometimes returns JSON objects separated by newlines
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var s struct {
			ID   string `json:"ID"`
			Name string `json:"Name"`
		}
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			continue
		}

		// Filter: We are only interested in secrets for this stack
		if strings.HasPrefix(s.Name, stackPrefix+"_") {
			secrets = append(secrets, SecretInfo{ID: s.ID, Name: s.Name})
		}
	}
	return secrets, nil
}

func getUsedSecretIDs(ctx context.Context, stackName string) (map[string]bool, error) {
	// 1. Get IDs of all services in the stack
	cmd := exec.CommandContext(ctx, "docker", "stack", "services", stackName, "--format", "{{.ID}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	serviceIDs := strings.Split(strings.TrimSpace(string(out)), "\n")
	var validIDs []string
	for _, id := range serviceIDs {
		if strings.TrimSpace(id) != "" {
			validIDs = append(validIDs, strings.TrimSpace(id))
		}
	}

	used := make(map[string]bool)
	if len(validIDs) == 0 {
		return used, nil
	}

	// 2. Retrieve secret list for each service
	args := append([]string{"service", "inspect", "--format", "{{json .Spec.TaskTemplate.ContainerSpec.Secrets}}"}, validIDs...)
	cmdInspect := exec.CommandContext(ctx, "docker", args...)
	outInspect, err := cmdInspect.Output()
	if err != nil {
		return nil, fmt.Errorf("inspect services: %w", err)
	}

	lines := strings.Split(string(outInspect), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "null" {
			continue
		}

		var serviceSecrets []struct {
			SecretID string `json:"SecretID"`
		}

		if err := json.Unmarshal([]byte(line), &serviceSecrets); err != nil {
			return nil, fmt.Errorf("parse inspect line '%s': %w", line, err)
		}

		for _, s := range serviceSecrets {
			if s.SecretID != "" {
				used[s.SecretID] = true
			}
		}
	}

	return used, nil
}

func removeSecret(ctx context.Context, id string) error {
	return exec.CommandContext(ctx, "docker", "secret", "rm", id).Run()
}
