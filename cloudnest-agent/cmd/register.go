package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudnest/cloudnest-agent/internal/agent"
	"github.com/cloudnest/cloudnest-agent/internal/storage"
	"github.com/spf13/cobra"
)

var (
	masterURL string
	regToken  string
	tokenFile string
	agentPort int
	scanDirs  []string
	rateLimit int64
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register this agent with the master server",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := resolveRegistrationToken(regToken, tokenFile)
		if err != nil {
			return err
		}

		cfg := &agent.Config{
			MasterURL: masterURL,
			Port:      agentPort,
			ScanDirs:  scanDirs,
			RateLimit: rateLimit,
		}
		if err := agent.RegisterWithMaster(cfg, token); err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
		log.Println("Agent registered successfully")
		return nil
	},
}

func init() {
	registerCmd.Flags().StringVar(&masterURL, "master", "http://localhost:8800", "Master server URL")
	registerCmd.Flags().StringVar(&regToken, "token", "", "Registration token")
	registerCmd.Flags().StringVar(&tokenFile, "token-file", "", "Path to a file containing registration token")
	registerCmd.Flags().IntVar(&agentPort, "port", 8801, "Agent data plane port")
	registerCmd.Flags().StringSliceVar(&scanDirs, "scan-dirs", []string{storage.FilesDir()}, "Directories to scan for file tree")
	registerCmd.Flags().Int64Var(&rateLimit, "rate-limit", 0, "Transfer rate limit in bytes/s (0=unlimited)")
	rootCmd.AddCommand(registerCmd)
}

func resolveRegistrationToken(tokenValue, tokenFilePath string) (string, error) {
	if strings.TrimSpace(tokenValue) != "" {
		return strings.TrimSpace(tokenValue), nil
	}
	if strings.TrimSpace(tokenFilePath) == "" {
		return "", fmt.Errorf("either --token or --token-file is required")
	}

	data, err := os.ReadFile(tokenFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read --token-file %q: %w", tokenFilePath, err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("--token-file %q is empty", tokenFilePath)
	}
	return token, nil
}
