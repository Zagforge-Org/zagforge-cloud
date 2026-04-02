package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/shared/go/jobtoken"
	"github.com/LegationPro/zagforge/shared/go/middleware/zaplogger"
	"github.com/LegationPro/zagforge/shared/go/middleware/zaprecoverer"
	"github.com/LegationPro/zagforge/shared/go/store"
	"github.com/LegationPro/zagforge/worker/internal/worker/config"
	"github.com/LegationPro/zagforge/worker/internal/worker/executor"
	"github.com/LegationPro/zagforge/worker/internal/worker/handler"
)

func runHTTP(ctx context.Context, cfg *config.Config, queries *store.Queries, exec *executor.Executor, signer *jobtoken.Signer, log *zap.Logger) error {
	h := handler.New(queries, exec, signer, log)

	r := chi.NewRouter()

	// Global middleware stack.
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(zaplogger.Middleware(log))
	r.Use(zaprecoverer.Middleware(log))
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.Throttle(cfg.MaxConcurrency))

	r.Post("/run", h.Run)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Info("worker http server listening", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()

	log.Info("shutting down worker http server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Info("worker http server stopped")
	return nil
}
