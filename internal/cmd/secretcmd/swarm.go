package secretcmd

import (
	"fmt"

	"github.com/rollwave-dev/rollwave/internal/config"
	"github.com/rollwave-dev/rollwave/internal/secrets"
	"github.com/spf13/cobra"
)

func newSwarmCmd() *cobra.Command {
	var (
		flagStack      string
		flagPrefix     string
		flagDryRun     bool
		flagConfigPath string
		flagEnv        string
	)

	c := &cobra.Command{
		Use:   "swarm",
		Short: "Sync Rollwave secrets into Docker Swarm",
		Long: `Reads ROLLWAVE_SECRET_* from environment (and .env if loaded)
and creates/updates Docker Swarm secrets for a given stack.

Example:
  # Using config (recommended)
  rollwave secrets swarm --env staging

  # Manual override (legacy style)
  rollwave secrets swarm --stack myapp --prefix prod`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Try to load config (optional)
			cfgPath := flagConfigPath
			if cfgPath == "" {
				cfgPath = "rollwave.yml"
			}

			var stackName, stackPrefix string

			// We attempt to load config, but don't fail if it's missing
			// UNLESS the user didn't provide --stack flag.
			baseCfg, err := config.Load(cfgPath)
			if err == nil {
				// Config loaded, apply env
				cfg, err := baseCfg.MergeWithEnv(flagEnv)
				if err != nil {
					return err
				}
				stackName = cfg.Stack.Name
				stackPrefix = cfg.Secrets.StackPrefix

				if flagEnv != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "🌍 Using environment: %s\n", flagEnv)
				}
			}

			// 2. Flags override config
			if flagStack != "" {
				stackName = flagStack
			}
			if flagPrefix != "" {
				stackPrefix = flagPrefix
			}

			// 3. Validation
			if stackName == "" {
				return fmt.Errorf("stack name is required (provide via --stack or rollwave.yml)")
			}

			_, err = secrets.EnsureSecrets(cmd.Context(), secrets.SyncOptions{
				Stack:  stackName,
				Prefix: stackPrefix,
				DryRun: flagDryRun,
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
			return err
		},
	}

	c.Flags().StringVar(&flagStack, "stack", "", "Docker Swarm stack name (overrides config)")
	c.Flags().StringVar(&flagPrefix, "prefix", "", "Optional extra prefix for secret names")
	c.Flags().BoolVar(&flagDryRun, "dry-run", false, "Show what would be changed without applying")
	c.Flags().StringVarP(&flagConfigPath, "config", "c", "", "Path to rollwave.yml")
	c.Flags().StringVarP(&flagEnv, "env", "e", "", "Environment to use (e.g. staging)")

	return c
}
