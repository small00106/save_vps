package cmd

import (
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
	Run: func(cmd *cobra.Command, args []string) {
		cfg := &agent.Config{
			MasterURL: masterURL,
			Port:      agentPort,
			ScanDirs:  scanDirs,
			RateLimit: rateLimit,
		}
		if err := agent.RegisterWithMaster(cfg, regToken); err != nil {
			log.Fatalf("Registration failed: %v", err)
		}
		log.Println("Agent registered successfully")
	},
}

func init() {
	registerCmd.Flags().StringVar(&masterURL, "master", "http://localhost:8800", "Master server URL")
	registerCmd.Flags().StringVar(&regToken, "token", "cloudnest-register", "Registration token")
	registerCmd.Flags().IntVar(&agentPort, "port", 8801, "Agent data plane port")
	registerCmd.Flags().StringSliceVar(&scanDirs, "scan-dirs", []string{storage.FilesDir()}, "Directories to scan for file tree")
	registerCmd.Flags().Int64Var(&rateLimit, "rate-limit", 0, "Transfer rate limit in bytes/s (0=unlimited)")
	rootCmd.AddCommand(registerCmd)
}
