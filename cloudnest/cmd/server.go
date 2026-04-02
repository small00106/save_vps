package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	// 1. Initialize database
	if err := dbcore.Init(dbType, dbDSN); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Database initialized")

	// 2. Initialize cache
	cache.Init()

	// 3. Set signing secret from env
	if secret := os.Getenv("CLOUDNEST_SIGNING_SECRET"); secret != "" {
		transfer.SetSigningSecret(secret)
	}

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
