package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	agentAPI "github.com/cloudnest/cloudnest/internal/api/agent"
	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/scheduler"
	"github.com/cloudnest/cloudnest/internal/server"
	"github.com/cloudnest/cloudnest/internal/transfer"
	"github.com/spf13/cobra"
)

var (
	listenAddr string
	dbType     string
	dbDSN      string
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the CloudNest master server",
	Run:   runServer,
}

func init() {
	serverCmd.Flags().StringVarP(&listenAddr, "listen", "l", getEnv("CLOUDNEST_LISTEN", "0.0.0.0:8800"), "Listen address")
	serverCmd.Flags().StringVar(&dbType, "db-type", getEnv("CLOUDNEST_DB_TYPE", "sqlite"), "Database type: sqlite or mysql")
	serverCmd.Flags().StringVar(&dbDSN, "db-dsn", getEnv("CLOUDNEST_DB_DSN", "./data/cloudnest.db"), "Database DSN")
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) {
	resolvedDBType, resolvedDBDSN, warning, err := resolveDBConfig("", dbType, dbDSN)
	if err != nil {
		log.Fatalf("Failed to resolve database config: %v", err)
	}
	if warning != "" {
		log.Printf("WARNING: %s", warning)
	}
	log.Printf("Using database config: dbType=%s dbDSN=%s", resolvedDBType, resolvedDBDSN)

	dataDir, err := resolveDataDir("", resolvedDBType, resolvedDBDSN)
	if err != nil {
		log.Fatalf("Failed to resolve data directory: %v", err)
	}
	secrets, err := resolveRuntimeSecrets(dataDir, os.Getenv)
	if err != nil {
		log.Fatalf("Failed to resolve runtime secrets: %v", err)
	}

	// 1. Initialize database
	if err := dbcore.Init(resolvedDBType, resolvedDBDSN); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Database initialized")

	// 2. Initialize cache
	cache.Init()

	// 3. Set runtime secrets
	transfer.SetSigningSecret(secrets.SigningSecret)
	agentAPI.SetRegistrationToken(secrets.RegistrationToken)

	// 4. Start background schedulers
	scheduler.StartAll()
	log.Println("Background schedulers started")

	// 4. Create HTTP server
	router := server.SetupRouter()
	srv := &http.Server{
		Addr:    listenAddr,
		Handler: router,
	}

	// 5. Start server in goroutine
	go func() {
		log.Printf("CloudNest master server starting on %s", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 6. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println()
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scheduler.StopAll()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func resolveDBConfig(executablePath, configuredType, configuredDSN string) (string, string, string, error) {
	resolvedType := "sqlite"
	if configuredType == "mysql" {
		resolvedType = "mysql"
	}

	if resolvedType != "sqlite" || configuredDSN == "" || configuredDSN == ":memory:" || filepath.IsAbs(configuredDSN) {
		return resolvedType, configuredDSN, "", nil
	}

	execDir, err := executableDir(executablePath)
	if err != nil {
		return "", "", "", err
	}

	resolvedDSN, err := filepath.Abs(filepath.Join(execDir, configuredDSN))
	if err != nil {
		return "", "", "", fmt.Errorf("resolve sqlite db path %q: %w", configuredDSN, err)
	}

	warning := fmt.Sprintf("sqlite db-dsn %q is relative; resolved against executable directory to %q", configuredDSN, resolvedDSN)
	return resolvedType, resolvedDSN, warning, nil
}

func executableDir(executablePath string) (string, error) {
	if executablePath == "" {
		var err error
		executablePath, err = os.Executable()
		if err != nil {
			return "", fmt.Errorf("resolve executable path: %w", err)
		}
	}

	execDir := filepath.Dir(executablePath)
	if execDir == "." || execDir == "" {
		return "", fmt.Errorf("resolve executable directory from %q", executablePath)
	}
	return execDir, nil
}
