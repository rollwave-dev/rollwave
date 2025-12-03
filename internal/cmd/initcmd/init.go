package initcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new rollwave.yml config",
		RunE: func(cmd *cobra.Command, args []string) error {

			path := "rollwave.yml"

			// Template reflects the new architecture:
			// 1. No build config here (it lives in docker-compose.yml)
			// 2. Defined stack and secret prefix
			example := `version: v1
project: my-new-project

stack:
  name: my-stack
  compose_file: docker-compose.yml

secrets:
  # Prefix ensures your secrets don't clash with other stacks (e.g. prod, staging)
  stack_prefix: prod

deploy:
  # If true, ROLLWAVE_SECRET_* vars are synced to Swarm before deploy
  with_secrets: true
`

			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("%s already exists", path)
			}

			if err := os.WriteFile(filepath.Clean(path), []byte(example), 0644); err != nil {
				return err
			}

			fmt.Println("✅ Created rollwave.yml")
			fmt.Println("👉 Next steps:")
			fmt.Println("   1. Edit rollwave.yml to match your project name.")
			fmt.Println("   2. Ensure your docker-compose.yml has 'image' and 'build' sections.")
			fmt.Println("   3. Run 'rollwave deploy --build'")
			return nil
		},
	}

	return cmd
}
