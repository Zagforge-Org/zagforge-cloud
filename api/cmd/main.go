package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/LegationPro/zagforge-mvp-impl/api/internal/config"
	"github.com/LegationPro/zagforge-mvp-impl/api/internal/db"
	"github.com/LegationPro/zagforge-mvp-impl/api/internal/handler"
	"github.com/LegationPro/zagforge-mvp-impl/api/internal/runner"
	"github.com/LegationPro/zagforge-mvp-impl/api/internal/service"
	githubprovider "github.com/LegationPro/zagforge-mvp-impl/shared/go/provider/github"
	"github.com/go-chi/chi/v5"
)

func main() {
	c, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	pool, err := db.Connect(context.Background(), c.DB.URL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	database := db.New(pool)

	client, err := githubprovider.NewAPIClient(c.App.GithubAppID, []byte(c.App.GithubAppPrivateKey), c.App.GithubAppWebhookSecret)
	if err != nil {
		log.Fatalf("failed to create API client: %v", err)
	}

	ch, err := githubprovider.NewClientHandler(client)
	if err != nil {
		log.Fatalf("failed to create client handler: %v", err)
	}

	run := runner.New(ch, runner.Config{
		WorkspaceDir: c.Worker.WorkspaceDir,
		ZigzagBin:    c.Worker.ZigzagBin,
		ReportsDir:   c.Worker.ReportsDir,
	})

	svc := service.NewJobService(database, run)
	wh := handler.NewWebhookHandler(ch, svc)

	mux := chi.NewRouter()
	mux.Post("/internal/webhooks/github", wh.ServeHTTP)

	srv := &http.Server{
		Addr:    ":" + c.Server.Port,
		Handler: mux,
	}

	go func() {
		log.Printf("server listening on :%s", c.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()

	log.Println("shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}

	log.Println("waiting for in-flight jobs to complete...")
	run.Wait()
	log.Println("server stopped")
}
