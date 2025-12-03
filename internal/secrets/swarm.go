package secrets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// SyncOptions defines options for secret synchronization.
type SyncOptions struct {
	Stack  string
	Prefix string
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
}

// SecretMap maps the logical name (from docker-compose) to the physical name (in Swarm).
type SecretMap map[string]string

// EnsureSecrets creates new secret versions if they don't exist and returns the mapping.
func EnsureSecrets(ctx context.Context, opt SyncOptions) (SecretMap, error) {
	if opt.Stdout == nil {
		opt.Stdout = os.Stdout
	}
	if opt.Stderr == nil {
		opt.Stderr = os.Stderr
	}

	loadedSecrets, err := Load()
	if err != nil {
		return nil, err
	}

	mapping := make(SecretMap)

	if len(loadedSecrets) == 0 {
		return mapping, nil
	}

	for _, s := range loadedSecrets {
		// 1. Calculate content hash (first 8 chars are sufficient)
		hash := hashString(s.Value)[:8]

		// 2. Construct physical name: stack_prefix_key_hash
		physicalName := buildSwarmSecretName(opt.Stack, opt.Prefix, s.Key, hash)

		// 3. Save to map: key (as known in compose) -> value (as in Swarm)
		mapping[s.Key] = physicalName

		if opt.DryRun {
			fmt.Fprintf(opt.Stdout, "[dry-run] ensure secret %s\n", physicalName)
			continue
		}

		// 4. Create secret only if it doesn't exist (idempotency)
		if !secretExists(ctx, physicalName) {
			if err := createSecret(ctx, physicalName, s.Value); err != nil {
				return nil, fmt.Errorf("failed to create secret %s: %w", physicalName, err)
			}
			fmt.Fprintf(opt.Stdout, "Created new secret version: %s\n", physicalName)
		} else {
			// Secret already exists, do nothing (immutable)
		}
	}

	return mapping, nil
}

// -----------------------------------------------------------------------------
// Helper functions
// -----------------------------------------------------------------------------

func hashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func buildSwarmSecretName(stack, prefix, key, hash string) string {
	// Base format: stack_prefix_key_hash
	parts := []string{stack}
	if prefix != "" {
		parts = append(parts, prefix)
	}
	parts = append(parts, key, hash)
	return strings.Join(parts, "_")
}

func secretExists(ctx context.Context, name string) bool {
	// Quick check using inspect
	cmd := exec.CommandContext(ctx, "docker", "secret", "inspect", name)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func createSecret(ctx context.Context, name, value string) error {
	cmd := exec.CommandContext(ctx, "docker", "secret", "create", name, "-")
	cmd.Stdin = strings.NewReader(value)
	cmd.Stdout = io.Discard
	return cmd.Run()
}
