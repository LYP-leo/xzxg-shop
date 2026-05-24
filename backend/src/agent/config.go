package agent

import "os"

type RuntimeConfig struct {
	Models ModelConfig
}

func RuntimeConfigFromEnv() RuntimeConfig {
	return RuntimeConfig{
		Models: ModelConfig{
			BaseURL:        env("AI_BASE_URL", env("DASHSCOPE_BASE_URL", defaultDashScopeBaseURL)),
			APIKey:         env("DASHSCOPE_API_KEY", env("AI_API_KEY", "")),
			SmallModel:     env("AI_SMALL_MODEL", "qwen3.5-flash"),
			LargeModel:     env("AI_LARGE_MODEL", "qwen3.6-plus"),
			EnableThinking: parseEnvBool(env("AI_ENABLE_THINKING", "false"), false),
		},
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func parseEnvBool(value string, fallback bool) bool {
	switch value {
	case "true", "1", "yes", "on", "enabled":
		return true
	case "false", "0", "no", "off", "disabled":
		return false
	default:
		return fallback
	}
}
