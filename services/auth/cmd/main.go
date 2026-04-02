package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/docgen"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/config"
	"github.com/LegationPro/zagforge/auth/internal/routes"
	"github.com/LegationPro/zagforge/shared/go/logger"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func run() error {
	c, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := logger.New(os.Getenv("APP_ENV"))
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	d, cleanup, err := initDeps(context.Background(), c, log)
	if err != nil {
		return err
	}
	defer cleanup()

	rd := newRouteDeps(d, c, log)

	r := router.New()
	if err := routes.Register(r, rd); err != nil {
		return err
	}

	if os.Getenv("APP_ENV") == "dev" {
		docgen.PrintRoutes(r.Mux())
	}

	srv := &http.Server{
		Addr:    ":" + c.Server.Port,
		Handler: r.Handler(),
	}

	go func() {
		log.Info("auth server listening", zap.String("port", c.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()

	log.Info("shutting down auth server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Info("auth server stopped")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
