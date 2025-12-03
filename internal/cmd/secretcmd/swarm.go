package secretcmd

import (
	"github.com/spf13/cobra"

	"github.com/rollwave-dev/rollwave/internal/secrets"
)

func newSwarmCmd() *cobra.Command {
	var stack string
	var prefix string
	var dryRun bool

	c := &cobra.Command{
		Use:   "swarm",
		Short: "Sync Rollwave secrets into Docker Swarm",
		Long: `Reads ROLLWAVE_SECRET_* from environment (and .env if loaded)
and creates/updates Docker Swarm secrets for a given stack.

Example:
  ROLLWAVE_SECRET_DB_PASSWORD=supersecret
  rollwave secrets swarm --stack myapp

→ docker secret create myapp_DB_PASSWORD ...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := secrets.EnsureSecrets(cmd.Context(), secrets.SyncOptions{
				Stack:  stack,
				Prefix: prefix,
				DryRun: dryRun,
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
			return err
		},
	}

	c.Flags().StringVar(&stack, "stack", "", "Docker Swarm stack name (required)")
	c.Flags().StringVar(&prefix, "prefix", "", "Optional extra prefix for secret names")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be changed without applying")
	_ = c.MarkFlagRequired("stack")

	return c
}
