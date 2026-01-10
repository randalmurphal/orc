package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/api"
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

Example:
  orc serve              # Start on default port 8080
  orc serve --port 3000  # Start on custom port`,
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")
			addr := fmt.Sprintf(":%d", port)

			cfg := &api.Config{
				Addr: addr,
			}

			server := api.New(cfg)

			fmt.Printf("Starting API server on http://localhost%s\n", addr)
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

	return cmd
}
