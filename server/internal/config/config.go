package config

import (
	"errors"
	"strings"

	"github.com/spf13/viper"
)

const (
	defaultServerPort         = "8080"
	defaultLogLevel           = "info"
	defaultRedisDB            = 0
	defaultRedisRefreshPrefix = "auth:refresh"
	defaultRedisRankingPrefix = "ranking:v1"
)

var defaultSensitiveFields = []string{"authorization", "cookie", "set-cookie", "password", "token", "refresh_token", "refreshToken"}

// RuntimeConfig stores all runtime settings needed by API startup.
//
// Values are loaded from environment variables and optional local config files
// to keep startup behavior deterministic and testable.
type RuntimeConfig struct {
	ServerPort         string
	JWTSecret          string
	MySQLDSN           string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	RedisRefreshPrefix string
	RedisRankingPrefix string
	LogLevel           string
	LogSensitiveFields []string
}

// Load resolves runtime config with Viper.
//
// Priority: environment variables > config file > defaults.
func Load() (RuntimeConfig, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName("app")
	v.AddConfigPath("./server/configs")
	v.AddConfigPath("./configs")

	v.SetDefault("SERVER_PORT", defaultServerPort)
	v.SetDefault("LOG_LEVEL", defaultLogLevel)
	v.SetDefault("REDIS_DB", defaultRedisDB)
	v.SetDefault("REDIS_REFRESH_PREFIX", defaultRedisRefreshPrefix)
	v.SetDefault("REDIS_RANKING_PREFIX", defaultRedisRankingPrefix)

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return RuntimeConfig{}, err
		}
	}

	cfg := RuntimeConfig{
		ServerPort:         strings.TrimSpace(v.GetString("SERVER_PORT")),
		JWTSecret:          strings.TrimSpace(v.GetString("JWT_SECRET")),
		MySQLDSN:           strings.TrimSpace(v.GetString("MYSQL_DSN")),
		RedisAddr:          strings.TrimSpace(v.GetString("REDIS_ADDR")),
		RedisPassword:      strings.TrimSpace(v.GetString("REDIS_PASSWORD")),
		RedisDB:            v.GetInt("REDIS_DB"),
		RedisRefreshPrefix: strings.TrimSpace(v.GetString("REDIS_REFRESH_PREFIX")),
		RedisRankingPrefix: strings.TrimSpace(v.GetString("REDIS_RANKING_PREFIX")),
		LogLevel:           strings.TrimSpace(v.GetString("LOG_LEVEL")),
	}

	if cfg.ServerPort == "" {
		cfg.ServerPort = defaultServerPort
	}
	if cfg.JWTSecret == "" {
		return RuntimeConfig{}, errors.New("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return RuntimeConfig{}, errors.New("JWT_SECRET must be at least 32 characters")
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaultLogLevel
	}
	if cfg.RedisRefreshPrefix == "" {
		cfg.RedisRefreshPrefix = defaultRedisRefreshPrefix
	}
	if cfg.RedisRankingPrefix == "" {
		cfg.RedisRankingPrefix = defaultRedisRankingPrefix
	}

	cfg.LogSensitiveFields = parseSensitiveFields(v.GetString("LOG_SENSITIVE_FIELDS"))
	return cfg, nil
}

func parseSensitiveFields(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		out := make([]string, len(defaultSensitiveFields))
		copy(out, defaultSensitiveFields)
		return out
	}

	parts := strings.Split(raw, ",")
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		fields = append(fields, name)
	}
	if len(fields) == 0 {
		out := make([]string, len(defaultSensitiveFields))
		copy(out, defaultSensitiveFields)
		return out
	}
	return fields
}
