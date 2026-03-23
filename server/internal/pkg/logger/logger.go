package logger

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
)

const (
	EnvLogLevel        = "LOG_LEVEL"
	EnvSensitiveFields = "LOG_SENSITIVE_FIELDS"

	maskedValue = "***"
)

var (
	defaultSensitiveFields = []string{"authorization", "cookie", "set-cookie", "password", "token", "refresh_token", "refreshToken"}

	mu           sync.RWMutex
	global       *zap.Logger
	sensitiveSet map[string]struct{}
)

func init() {
	if err := Init("info"); err != nil {
		global = zap.NewNop()
	}
	SetSensitiveFields(defaultSensitiveFields)
}

func Init(level string) error {
	cfg := zap.NewProductionConfig()
	parsedLevel := zap.InfoLevel
	if err := parsedLevel.Set(strings.TrimSpace(strings.ToLower(level))); err != nil {
		return err
	}
	cfg.Level = zap.NewAtomicLevelAt(parsedLevel)

	newLogger, err := cfg.Build(zap.AddCaller(), zap.AddCallerSkip(1))
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	if global != nil {
		_ = global.Sync()
	}
	global = newLogger
	return nil
}

func InitFromEnv() {
	level := strings.TrimSpace(os.Getenv(EnvLogLevel))
	if level == "" {
		level = "info"
	}
	if err := Init(level); err != nil {
		_ = Init("info")
	}

	rawSensitive := strings.TrimSpace(os.Getenv(EnvSensitiveFields))
	if rawSensitive == "" {
		SetSensitiveFields(defaultSensitiveFields)
		return
	}

	parts := strings.Split(rawSensitive, ",")
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		fields = append(fields, name)
	}
	if len(fields) == 0 {
		SetSensitiveFields(defaultSensitiveFields)
		return
	}
	SetSensitiveFields(fields)
}

func L() *zap.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if global == nil {
		return zap.NewNop()
	}
	return global
}

func SetSensitiveFields(fields []string) {
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		normalized := strings.ToLower(strings.TrimSpace(field))
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}

	mu.Lock()
	defer mu.Unlock()
	sensitiveSet = set
}

func SetLoggerForTest(testLogger *zap.Logger, sensitiveFields []string) func() {
	mu.Lock()
	prevLogger := global
	prevSensitive := cloneSensitiveSet(sensitiveSet)
	global = testLogger
	mu.Unlock()

	SetSensitiveFields(sensitiveFields)

	return func() {
		mu.Lock()
		global = prevLogger
		sensitiveSet = prevSensitive
		mu.Unlock()
	}
}

func SanitizeHeaders(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		joined := strings.Join(values, ",")
		if isSensitiveKey(key) {
			result[key] = maskedValue
			continue
		}
		result[key] = joined
	}
	return result
}

func SanitizeQuery(values url.Values) map[string]string {
	result := make(map[string]string, len(values))
	for key, list := range values {
		joined := strings.Join(list, ",")
		if isSensitiveKey(key) {
			result[key] = maskedValue
			continue
		}
		result[key] = joined
	}
	return result
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	mu.RLock()
	defer mu.RUnlock()
	_, ok := sensitiveSet[normalized]
	return ok
}

func cloneSensitiveSet(source map[string]struct{}) map[string]struct{} {
	if source == nil {
		return map[string]struct{}{}
	}
	cloned := make(map[string]struct{}, len(source))
	for key := range source {
		cloned[key] = struct{}{}
	}
	return cloned
}
