package cmd

import (
	"fmt"
	"log"

	"github.com/cloudnest/cloudnest-agent/internal/agent"
	"github.com/cloudnest/cloudnest-agent/internal/storage"
	"github.com/spf13/cobra"
)

var (
	masterURL string
	regToken  string
	agentPort int
	scanDirs  []string
	rateLimit int64
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register this agent with the master server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := &agent.Config{
			MasterURL: masterURL,
			Port:      agentPort,
			ScanDirs:  scanDirs,
			RateLimit: rateLimit,
		}
		if err := agent.RegisterWithMaster(cfg, regToken); err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
		log.Println("Agent registered successfully")
		return nil
	},
}

func init() {
	registerCmd.Flags().StringVar(&masterURL, "master", "http://localhost:8800", "Master server URL")
	registerCmd.Flags().StringVar(&regToken, "token", "", "Registration token")
	registerCmd.Flags().IntVar(&agentPort, "port", 8801, "Agent data plane port")
	registerCmd.Flags().StringSliceVar(&scanDirs, "scan-dirs", []string{storage.FilesDir()}, "Directories to scan for file tree")
	registerCmd.Flags().Int64Var(&rateLimit, "rate-limit", 0, "Transfer rate limit in bytes/s (0=unlimited)")
	_ = registerCmd.MarkFlagRequired("token")
	rootCmd.AddCommand(registerCmd)
}
