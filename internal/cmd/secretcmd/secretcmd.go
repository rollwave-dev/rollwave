package secretcmd

import (
	"fmt"

	"github.com/rollwave-dev/rollwave/internal/secrets"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage Rollwave secrets",
	}

	// Default behavior: List secrets
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		secs, err := secrets.Load()
		if err != nil {
			return err
		}
		for _, s := range secs {
			fmt.Printf("%s=**** (len=%d)\n", s.Key, len(s.Value))
		}
		return nil
	}

	cmd.AddCommand(newSwarmCmd())

	return cmd
}
