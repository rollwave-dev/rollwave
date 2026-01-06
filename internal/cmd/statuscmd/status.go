package statuscmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rollwave-dev/rollwave/internal/config"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	var (
		flagConfigPath string
		flagEnv        string
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show deployment status of the stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Load Config
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
				return fmt.Errorf("stack name is missing in configuration")
			}

			// Header info
			fmt.Fprintf(cmd.OutOrStdout(), "🌍 Environment: %s\n", defaultEnvName(flagEnv))
			fmt.Fprintf(cmd.OutOrStdout(), "📦 Stack:       %s\n\n", stackName)

			return runStatus(cmd.Context(), stackName)
		},
	}

	cmd.Flags().StringVarP(&flagConfigPath, "config", "c", "", "Path to rollwave.yml")
	cmd.Flags().StringVarP(&flagEnv, "env", "e", "", "Environment to check (e.g. staging)")

	return cmd
}

func defaultEnvName(env string) string {
	if env == "" {
		return "production (default)"
	}
	return env
}

func runStatus(ctx context.Context, stackName string) error {
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}

	host := os.Getenv("DOCKER_HOST")
	if host != "" {
		helper, err := connhelper.GetConnectionHelper(host)
		if err != nil {
			return fmt.Errorf("ssh connection helper: %w", err)
		}
		if helper != nil {
			opts = append(opts, client.WithDialContext(helper.Dialer))
		}
	}

	cli, err := client.NewClientWithOpts(opts...)

	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}

	// 1. List Services for the Stack
	serviceFilter := filters.NewArgs()
	serviceFilter.Add("label", "com.docker.stack.namespace="+stackName)

	services, err := cli.ServiceList(ctx, types.ServiceListOptions{Filters: serviceFilter})
	if err != nil {
		return fmt.Errorf("list services: %w", err)
	}

	if len(services) == 0 {
		fmt.Println("⚠️  No services found for this stack.")
		return nil
	}

	// 2. List Tasks (to count running replicas)
	// We get all tasks for the stack to minimize API calls
	taskFilter := filters.NewArgs()
	taskFilter.Add("label", "com.docker.stack.namespace="+stackName)
	taskFilter.Add("desired-state", "running") // We care about tasks that should be running

	tasks, err := cli.TaskList(ctx, types.TaskListOptions{Filters: taskFilter})
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	// Map ServiceID -> Running Count
	runningCounts := make(map[string]int)
	for _, t := range tasks {
		if t.Status.State == "running" {
			runningCounts[t.ServiceID]++
		}
	}

	// 3. Print Table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tREPLICAS\tIMAGE\tPORTS")

	for _, svc := range services {
		// Clean service name (remove stack prefix)
		shortName := strings.TrimPrefix(svc.Spec.Name, stackName+"_")

		// Replicas (Running / Desired)
		desired := uint64(0)
		if svc.Spec.Mode.Replicated != nil && svc.Spec.Mode.Replicated.Replicas != nil {
			desired = *svc.Spec.Mode.Replicated.Replicas
		}
		// Note: Global mode doesn't have a fixed desired count in spec, handled simply here
		running := runningCounts[svc.ID]

		replicaStr := fmt.Sprintf("%d/%d", running, desired)
		if svc.Spec.Mode.Global != nil {
			replicaStr = fmt.Sprintf("%d (global)", running)
		}

		// Image
		image := svc.Spec.TaskTemplate.ContainerSpec.Image
		// Simplify image string (remove sha256 if too long)
		if idx := strings.Index(image, "@sha256"); idx != -1 {
			image = image[:idx]
		}

		// Ports
		var ports []string
		for _, p := range svc.Endpoint.Ports {
			ports = append(ports, fmt.Sprintf("%d->%d/%s", p.PublishedPort, p.TargetPort, p.Protocol))
		}
		portStr := strings.Join(ports, ", ")
		if portStr == "" {
			portStr = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", shortName, replicaStr, image, portStr)
	}

	w.Flush()
	return nil
}
