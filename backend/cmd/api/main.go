package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/agent"
	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/httpapi"
	"github.com/LYP-leo/xzxg-shop/backend/src/imagevector"
	"github.com/LYP-leo/xzxg-shop/backend/src/rag"
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
	production := isProductionEnv()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 后端当前只使用 MySQLStore；启动时先连库并执行兼容迁移。
	mysqlStore, err := store.OpenMySQL(ctx, dsn)
	if err != nil {
		logger.Error("mysql unavailable", "error", err)
		os.Exit(1)
	}
	defer mysqlStore.Close()
	// A production checkout may create an unpaid order, but cannot manufacture
	// a successful payment. Real payment must be completed by a provider.
	mysqlStore.SetMockPaymentsEnabled(!production && envBool("ENABLE_DEMO_PAYMENTS", true))
	if envBool("RUN_MIGRATIONS", !production) {
		if err := mysqlStore.Migrate(ctx); err != nil {
			logger.Error("mysql migration failed", "error", err)
			os.Exit(1)
		}
	}

	// Nacos 不可用时会退回内存默认配置，保证本地开发仍可启动。
	runtimeConfig := agent.RuntimeConfigFromEnv()
	configCenter := configcenter.NewNacosCenter(nacosAddr, nacosNamespace, nacosGroup, nacosDataID, configcenter.DefaultConfigs(runtimeConfig.Models.APIKey))
	if err := configCenter.Seed(ctx); err != nil {
		logger.Warn("nacos config center unavailable, using in-memory defaults", "error", err)
	}
	if production {
		if err := validateProductionConfig(dsn, configCenter.GetMap(ctx)); err != nil {
			logger.Error("unsafe production config", "error", err)
			os.Exit(1)
		}
	}
	if err := mysqlStore.SeedAgentPrompts(ctx, configcenter.PromptDefaults()); err != nil {
		logger.Warn("seed agent prompts failed", "error", err)
	}
	vectorConfig := rag.ConfigFromMap(configCenter.GetMap(ctx), runtimeConfig.Models.APIKey, runtimeConfig.Models.BaseURL)
	if vectorConfig.Enabled {
		mysqlStore.SetImageEmbedder(imagevector.NewEmbedderFromMap(configCenter.GetMap(ctx), runtimeConfig.Models.APIKey))
		vectorClient := rag.NewClient(vectorConfig, rag.NewOpenAIEmbedder(vectorConfig.EmbeddingBaseURL, vectorConfig.EmbeddingAPIKey, vectorConfig.EmbeddingModel))
		mysqlStore.SetVectorClient(vectorClient)
	}
	if vectorConfig.Enabled && envBool("BOOTSTRAP_VECTOR_INDEX", !production) {
		go func() {
			bootstrapCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			logger.Info("vector index bootstrap started", "milvus", vectorConfig.MilvusAddress)
			if err := mysqlStore.BootstrapVectorIndex(bootstrapCtx); err != nil {
				logger.Warn("vector index unavailable, falling back to mysql retrieval", "error", err)
				return
			}
			logger.Info("vector index ready", "milvus", vectorConfig.MilvusAddress)
			imageBootstrapCtx, imageCancel := context.WithTimeout(ctx, 20*time.Minute)
			defer imageCancel()
			logger.Info("image vector index bootstrap started", "milvus", vectorConfig.MilvusAddress, "collection", vectorConfig.ImageCollection)
			if err := mysqlStore.BootstrapImageVectorIndex(imageBootstrapCtx); err != nil {
				logger.Warn("image vector index unavailable", "error", err)
				return
			}
			logger.Info("image vector index ready", "milvus", vectorConfig.MilvusAddress, "collection", vectorConfig.ImageCollection)
		}()
	}
	runtime := agent.NewRuntime(mysqlStore, configCenter, logger, runtimeConfig)
	server := httpapi.NewServer(mysqlStore, configCenter, runtime, logger)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			batchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			closed, err := mysqlStore.ExpirePendingOrdersBatch(batchCtx)
			cancel()
			if err != nil && ctx.Err() == nil {
				logger.Error("order expiry failed; will retry", "closed", closed, "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

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

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on", "enabled":
		return true
	case "0", "false", "no", "off", "disabled":
		return false
	default:
		return fallback
	}
}

func isProductionEnv() bool {
	value := strings.ToLower(strings.TrimSpace(env("APP_ENV", env("GO_ENV", ""))))
	return value == "prod" || value == "production"
}

func validateProductionConfig(dsn string, values map[string]string) error {
	lowerDSN := strings.ToLower(dsn)
	switch {
	case strings.HasPrefix(lowerDSN, "root:root@"):
		return errors.New("MYSQL_DSN must not use root:root in production")
	case strings.Contains(values["http.cors.allowed_origins"], "*"):
		return errors.New("http.cors.allowed_origins must be explicit in production")
	case strings.TrimSpace(values["minio.access_key"]) == "" || values["minio.access_key"] == "minioadmin":
		return errors.New("minio.access_key must be configured in production")
	case strings.TrimSpace(values["minio.secret_key"]) == "" || values["minio.secret_key"] == "minioadmin":
		return errors.New("minio.secret_key must be configured in production")
	case strings.TrimSpace(values["milvus.token"]) == "" || values["milvus.token"] == "root:Milvus":
		return errors.New("milvus.token must be configured in production")
	case strings.TrimSpace(values["ai.qwen.api_key"]) == "":
		return errors.New("ai.qwen.api_key must be configured in production")
	}
	return nil
}

func emptyFallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
