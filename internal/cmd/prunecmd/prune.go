package prunecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rollwave-dev/rollwave/internal/config"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	var flagConfigPath string

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove unused secrets for the stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Load Config
			cfgPath := flagConfigPath
			if cfgPath == "" {
				cfgPath = "rollwave.yml"
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			stackName := cfg.Stack.Name
			if stackName == "" {
				return fmt.Errorf("stack name required")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "🧹 Pruning secrets for stack '%s'...\n", stackName)

			// 2. Get all secrets belonging to this project (by prefix)
			secrets, err := listSecrets(cmd.Context(), stackName)
			if err != nil {
				return err
			}

			// 3. Get list of secrets CURRENTLY used by services
			usedSecretIDs, err := getUsedSecretIDs(cmd.Context(), stackName)
			if err != nil {
				return err
			}

			// 4. Compare and delete
			deletedCount := 0
			for _, s := range secrets {
				// If secret ID is not in the used list -> DELETE
				if !usedSecretIDs[s.ID] {
					fmt.Fprintf(cmd.OutOrStdout(), "   Deleting unused secret: %s\n", s.Name)
					if err := removeSecret(cmd.Context(), s.ID); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "   ⚠️ Failed to remove %s: %v\n", s.Name, err)
					} else {
						deletedCount++
					}
				}
			}

			if deletedCount == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "✨ No unused secrets found. Clean.")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "🗑️  Deleted %d secrets.\n", deletedCount)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flagConfigPath, "config", "c", "", "Path to rollwave.yml")
	return cmd
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
	// Docker sometimes returns JSON objects separated by newlines, not a single array.
	// We must parse line by line.
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
		// Expected format: stackName_...
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
	// Filter empty lines
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
	// Docker prints JSON for each service on a new line
	args := append([]string{"service", "inspect", "--format", "{{json .Spec.TaskTemplate.ContainerSpec.Secrets}}"}, validIDs...)
	cmdInspect := exec.CommandContext(ctx, "docker", args...)
	outInspect, err := cmdInspect.Output()
	if err != nil {
		return nil, fmt.Errorf("inspect services: %w", err)
	}

	// 3. Parse line by line
	lines := strings.Split(string(outInspect), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "null" {
			continue
		}

		// Each line is a list of secrets for one service: [{"SecretID":"..."}, {"SecretID":"..."}]
		var serviceSecrets []struct {
			SecretID string `json:"SecretID"`
		}

		if err := json.Unmarshal([]byte(line), &serviceSecrets); err != nil {
			// If one line fails, return an error as it implies unexpected docker output
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
