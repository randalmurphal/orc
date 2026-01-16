package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/api"
	"github.com/randalmurphal/orc/internal/config"
)

// newServeCmd creates the serve command for the API server
func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the API server",
		Long: `Start the orc API server for the web UI.

The API server provides REST endpoints and SSE streaming for:
  • Task management (list, create, run, pause)
  • Live transcript streaming
  • State and plan queries

If the requested port is in use, the server will try subsequent ports
up to max-port-attempts times (default: 10). For example, if port 8080
is busy, it will try 8081, 8082, etc.

Example:
  orc serve              # Start on default port 8080
  orc serve --port 3000  # Start on custom port`,
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")
			maxPortAttempts, _ := cmd.Flags().GetInt("max-port-attempts")
			addr := fmt.Sprintf(":%d", port)

			// Load orc config for defaults
			orcCfg, err := config.Load()
			if err != nil {
				// Use defaults if config not available
				orcCfg = config.Default()
			}

			// Use config default if CLI flag not explicitly set
			if !cmd.Flags().Changed("max-port-attempts") {
				maxPortAttempts = orcCfg.Server.MaxPortAttempts
				if maxPortAttempts <= 0 {
					maxPortAttempts = 10
				}
			}

			cfg := &api.Config{
				Addr:            addr,
				MaxPortAttempts: maxPortAttempts,
			}

			server := api.New(cfg)

			fmt.Printf("Starting API server (port %d, will try up to %d ports if busy)...\n", port, maxPortAttempts)
			fmt.Println("Press Ctrl+C to stop")

			// Handle graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigCh
				fmt.Println("\nShutting down...")
				cancel()
			}()

			return server.StartContext(ctx)
		},
	}

	cmd.Flags().IntP("port", "p", 8080, "port to listen on")
	cmd.Flags().Int("max-port-attempts", 10, "max ports to try if initial port is busy")

	return cmd
}
