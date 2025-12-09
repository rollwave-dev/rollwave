package deploycmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/rollwave-dev/rollwave/internal/build"
	"github.com/rollwave-dev/rollwave/internal/compose"
	"github.com/rollwave-dev/rollwave/internal/config"
	"github.com/rollwave-dev/rollwave/internal/prune"
	"github.com/rollwave-dev/rollwave/internal/secrets"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	var (
		flagConfigPath  string
		flagWithSecrets bool
		flagBuild       bool
		flagEnv         string
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Build from Compose and deploy",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Load Base Config
			cfgPath := flagConfigPath
			if cfgPath == "" {
				cfgPath = "rollwave.yml"
			}
			baseCfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			// 2. Apply Environment Overrides
			cfg, err := baseCfg.MergeWithEnv(flagEnv)
			if err != nil {
				return err
			}

			if flagEnv != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "🌍 Using environment: %s\n", flagEnv)
			}

			// 3. Read Compose File
			composeFile := cfg.Stack.ComposeFile
			if composeFile == "" {
				composeFile = "docker-compose.yml"
			}
			originalYaml, err := os.ReadFile(composeFile)
			if err != nil {
				return fmt.Errorf("read compose file '%s': %w", composeFile, err)
			}

			currentYaml := originalYaml

			// ---------------------------------------------------------
			// PRE-CHECK: Analyze images & Login
			// (Execute always, even if not building, to ensure Login for Swarm)
			// ---------------------------------------------------------
			buildConfigs, err := compose.ExtractBuildConfigs(currentYaml)
			if err != nil {
				return err
			}

			// If services with images are defined, attempt to login
			// (So Swarm can pull the image even if build is skipped)
			if len(buildConfigs) > 0 {
				firstImage := buildConfigs[0].ImageName
				if err := build.Login(cmd.Context(), firstImage, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
					return fmt.Errorf("registry login: %w", err)
				}
			}

			// ---------------------------------------------------------
			// STEP A: BUILD (Based on Compose)
			// ---------------------------------------------------------
			if flagBuild {
				fmt.Fprintln(cmd.OutOrStdout(), "🏗️  Analyzing Compose file for builds...")

				if len(buildConfigs) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "⚠️  --build flag used, but no services have a 'build' section.")
				}

				imageReplacements := make(map[string]string)

				// Build and push each service
				for _, bConf := range buildConfigs {
					builtTag, err := build.Run(cmd.Context(), build.Options{
						ImageName:  bConf.ImageName,
						ContextDir: bConf.Context,
						Dockerfile: bConf.Dockerfile,
						Stdout:     cmd.OutOrStdout(),
						Stderr:     cmd.ErrOrStderr(),
					})
					if err != nil {
						return fmt.Errorf("build service %s: %w", bConf.ServiceName, err)
					}

					fmt.Printf("✅ Service '%s' built & pushed: %s\n", bConf.ServiceName, builtTag)
					imageReplacements[bConf.ServiceName] = builtTag
				}

				// Update YAML with new image tags
				currentYaml, err = compose.ReplaceImages(currentYaml, imageReplacements)
				if err != nil {
					return fmt.Errorf("replace images: %w", err)
				}
			}

			// ---------------------------------------------------------
			// STEP B: SECRETS
			// ---------------------------------------------------------
			withSecrets := flagWithSecrets
			// Check if CLI flag is set OR if config enables it (merged config)
			if !cmd.Flags().Changed("with-secrets") && cfg.Deploy.WithSecrets {
				withSecrets = true
			}

			if withSecrets {
				fmt.Fprintln(cmd.OutOrStdout(), "🔒 Ensuring secrets...")
				secretMap, err := secrets.EnsureSecrets(context.Background(), secrets.SyncOptions{
					Stack:  cfg.Stack.Name,
					Prefix: cfg.Secrets.StackPrefix,
					Stdout: cmd.OutOrStdout(),
				})
				if err != nil {
					return err
				}

				if len(secretMap) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "⚠️  WARNING: No secrets found beginning with ROLLWAVE_SECRET_")
				}

				currentYaml, err = compose.RewriteSecrets(currentYaml, secretMap)
				if err != nil {
					return err
				}
			}

			// ---------------------------------------------------------
			// STEP C: DEPLOY
			// ---------------------------------------------------------
			tempPath := "docker-compose.rollwave.generated.yml"
			if err := os.WriteFile(tempPath, currentYaml, 0644); err != nil {
				return err
			}
			defer os.Remove(tempPath)

			deployArgs := []string{
				"stack", "deploy",
				"--compose-file", tempPath,
				"--with-registry-auth",
				"--prune",
				cfg.Stack.Name,
			}

			fmt.Fprintf(cmd.OutOrStdout(), "🚀 Deploying stack '%s'...\n", cfg.Stack.Name)

			c := exec.CommandContext(cmd.Context(), "docker", deployArgs...)
			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.ErrOrStderr()

			// --- Inject variables into process environment ---
			// Docker CLI uses environment variables to substitute ${VAR} in compose files.
			c.Env = os.Environ() // Start with current environment (PATH, DOCKER_HOST, etc.)
			for k, v := range cfg.Variables {
				c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
				fmt.Fprintf(cmd.OutOrStdout(), "   Exporting var: %s=%s\n", k, v)
			}

			if err := c.Run(); err != nil {
				return fmt.Errorf("deploy failed: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "✅ Deployment successful.")

			// --- AUTO PRUNE ---
			if cfg.Deploy.Prune {
				fmt.Fprintln(cmd.OutOrStdout(), "") // New line for separation
				if err := prune.Run(cmd.Context(), cfg.Stack.Name, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
					// We don't fail the deployment if prune fails, just warn
					fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Auto-prune failed: %v\n", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flagConfigPath, "config", "c", "", "Path to rollwave.yml")
	cmd.Flags().BoolVar(&flagWithSecrets, "with-secrets", false, "Enable secret rotation")
	cmd.Flags().BoolVar(&flagBuild, "build", false, "Build services defined in docker-compose.yml")
	cmd.Flags().StringVarP(&flagEnv, "env", "e", "", "Environment to deploy to (e.g. staging, production)")

	return cmd
}
