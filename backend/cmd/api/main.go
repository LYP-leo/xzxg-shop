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
	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/httpapi"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// 运行时配置统一从环境变量读取，便于本地、Docker 和比赛部署复用同一入口。
	addr := env("API_ADDR", ":8080")
	dsn := env("MYSQL_DSN", "root:root@tcp(127.0.0.1:3306)/xzxg_shop?parseTime=true&loc=Local")
	nacosAddr := env("NACOS_ADDR", "http://127.0.0.1:8848")
	nacosNamespace := env("NACOS_NAMESPACE", "")
	nacosGroup := env("NACOS_GROUP", "XZXG_SHOP")
	nacosDataID := env("NACOS_DATA_ID", "xzxg-shop-app-config.json")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 后端当前只使用 MySQLStore；启动时先连库并执行兼容迁移。
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

	// Nacos 不可用时会退回内存默认配置，保证本地开发仍可启动。
	runtimeConfig := agent.RuntimeConfigFromEnv()
	configCenter := configcenter.NewNacosCenter(nacosAddr, nacosNamespace, nacosGroup, nacosDataID, configcenter.DefaultConfigs(runtimeConfig.Models.APIKey))
	if err := configCenter.Seed(ctx); err != nil {
		logger.Warn("nacos config center unavailable, using in-memory defaults", "error", err)
	}
	runtime := agent.NewRuntime(mysqlStore, configCenter, logger, runtimeConfig)
	server := httpapi.NewServer(mysqlStore, configCenter, runtime, logger)

	// 所有 HTTP 中间件都在 Server.Routes 中组装，入口只负责生命周期。
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

	// 收到中断信号后给正在处理的请求最多 10 秒完成，避免直接断流。
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
