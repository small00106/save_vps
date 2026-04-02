package cmd

import (
	"log"

	"github.com/cloudnest/cloudnest-agent/internal/agent"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the agent (must register first)",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := agent.LoadConfig()
		if err != nil {
			log.Fatalf("Failed to load config: %v\nRun 'cloudnest-agent register' first.", err)
		}
		if err := agent.Run(cfg); err != nil {
			log.Fatalf("Agent error: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
