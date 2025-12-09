package prunecmd

import (
	"fmt"

	"github.com/rollwave-dev/rollwave/internal/config"
	"github.com/rollwave-dev/rollwave/internal/prune"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	var (
		flagConfigPath string
		flagEnv        string
	)

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove unused secrets for the stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Load Base Config
			cfgPath := flagConfigPath
			if cfgPath == "" {
				cfgPath = "rollwave.yml"
			}
			baseCfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// 2. Apply Environment Overrides
			cfg, err := baseCfg.MergeWithEnv(flagEnv)
			if err != nil {
				return err
			}

			stackName := cfg.Stack.Name
			if stackName == "" {
				return fmt.Errorf("stack name required in config")
			}

			if flagEnv != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "🌍 Using environment: %s\n", flagEnv)
			}

			// 3. Delegate to prune package
			return prune.Run(cmd.Context(), stackName, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringVarP(&flagConfigPath, "config", "c", "", "Path to rollwave.yml")
	cmd.Flags().StringVarP(&flagEnv, "env", "e", "", "Environment to prune (e.g. staging, production)")
	return cmd
}
