package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/agent"
	"github.com/LYP-leo/xzxg-shop/backend/src/httpapi"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := env("API_ADDR", ":8080")
	dsn := env("MYSQL_DSN", "root:root@tcp(127.0.0.1:3306)/xzxg_shop?parseTime=true&loc=Local")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mysqlStore, err := store.OpenMySQL(ctx, dsn)
	if err != nil {
		logger.Error("mysql unavailable", "error", err)
		os.Exit(1)
	}
	defer mysqlStore.Close()
	if err := mysqlStore.Migrate(ctx); err != nil {
		logger.Error("mysql migration failed", "error", err)
		os.Exit(1)
	}

	runtime := agent.NewRuntime(mysqlStore, logger, agent.RuntimeConfigFromEnv())
	server := httpapi.NewServer(mysqlStore, runtime, logger)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api listening", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("api shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("api shutdown completed")
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
