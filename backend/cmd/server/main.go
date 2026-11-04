package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/internal/app"
	"github.com/abysstract/abysstract-admin/backend/internal/buildinfo"
)

func main() {
	cfg := adminconfig.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	log.Printf("starting abysstract-admin %s", buildinfo.Current())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Println("shutdown signal received, draining connections...")
	}()

	if err := app.Run(ctx, *cfg); err != nil {
		if errors.Is(err, adminapp.ErrShutdownTimeout) {
			log.Printf("graceful shutdown failed, force closing: %v", err)
			time.Sleep(100 * time.Millisecond)
			os.Exit(1)
		}
		log.Fatalf("server error: %v", err)
	}
	log.Println("shutdown complete")
}
