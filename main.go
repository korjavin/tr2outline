package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC | log.Lshortfile)
	log.Println("Starting tr2outline webhook service...")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	outlineClient := NewOutlineClient(cfg.OutlineURL, cfg.OutlineAPIKey, cfg.OutlineCollectionID)
	server := NewWebhookServer(cfg, outlineClient)

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown channel
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on %s", addr)
		log.Printf("Webhook endpoint: POST http://localhost:%d/api/webhooks/anarlog", cfg.Port)
		log.Printf("Target Outline URL: %s, Collection ID: %s", cfg.OutlineURL, cfg.OutlineCollectionID)

		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-stopChan
	log.Println("Shutdown signal received, shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	} else {
		log.Println("Server stopped cleanly.")
	}
}
