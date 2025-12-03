package main

import (
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/rollwave-dev/rollwave/internal/cmd/deploycmd"
	"github.com/rollwave-dev/rollwave/internal/cmd/initcmd"
	"github.com/rollwave-dev/rollwave/internal/cmd/prunecmd"
	"github.com/rollwave-dev/rollwave/internal/cmd/secretcmd"
)

func main() {
	// Load .env if present
	_ = godotenv.Load()

	root := &cobra.Command{
		Use:   "rollwave",
		Short: "Rollwave",
	}

	root.AddCommand(initcmd.New())
	root.AddCommand(deploycmd.New())
	root.AddCommand(secretcmd.New())
	root.AddCommand(prunecmd.New())

	root.Execute()
}
