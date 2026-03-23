package router

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"CampusCanteenRank/server/internal/middleware"
	logpkg "CampusCanteenRank/server/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestTraceIDMiddlewareGeneratesHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TraceID())
	r.GET("/ping", func(c *gin.Context) {
		if middleware.GetTraceID(c) == "" {
			t.Fatalf("trace id should be set in context")
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Header().Get(middleware.TraceIDHeader) == "" {
		t.Fatalf("response should include %s", middleware.TraceIDHeader)
	}
}

func TestTraceIDMiddlewareKeepsIncomingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TraceID())
	r.GET("/ping", func(c *gin.Context) {
		if got := middleware.GetTraceID(c); got != "custom-trace-id" {
			t.Fatalf("trace id in context = %q, want custom-trace-id", got)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(middleware.TraceIDHeader, "custom-trace-id")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if got := rr.Header().Get(middleware.TraceIDHeader); got != "custom-trace-id" {
		t.Fatalf("response trace id = %q, want custom-trace-id", got)
	}
}

func TestTraceIDMiddlewareReplacesInvalidHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TraceID())
	r.GET("/ping", func(c *gin.Context) {
		traceID := middleware.GetTraceID(c)
		if strings.Contains(traceID, " ") {
			t.Fatalf("trace id should not contain space")
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(middleware.TraceIDHeader, "bad trace id")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if got := rr.Header().Get(middleware.TraceIDHeader); got == "bad trace id" {
		t.Fatalf("invalid incoming trace id should be replaced")
	}
}

func TestRecoverMiddlewareReturnsUnifiedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Recover())
	r.GET("/panic", func(_ *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	env := decodeEnvelope(t, rr.Body.Bytes())
	if got := asInt(t, env["code"]); got != 50000 {
		t.Fatalf("code = %d, want 50000", got)
	}
	if got, ok := env["message"].(string); !ok || got != "internal error" {
		t.Fatalf("message = %v, want internal error", env["message"])
	}
	if _, ok := env["data"].(map[string]any); !ok {
		t.Fatalf("data should be object")
	}
}

func TestMiddlewareChainLogsAndRecoversPanicWithTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, recorded := observer.New(zapcore.InfoLevel)
	testLogger := zap.New(core)
	restore := logpkg.SetLoggerForTest(testLogger, []string{"authorization", "token"})
	t.Cleanup(restore)

	r := gin.New()
	r.Use(middleware.TraceID())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recover())
	r.GET("/panic", func(_ *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set(middleware.TraceIDHeader, "trace-abc")
	req.Header.Set("Authorization", "Bearer secret-token")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	if got := rr.Header().Get(middleware.TraceIDHeader); got != "trace-abc" {
		t.Fatalf("trace id header = %q, want trace-abc", got)
	}

	var envelope map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil && err != io.EOF {
		t.Fatalf("decode response failed: %v", err)
	}
	if got := asInt(t, envelope["code"]); got != 50000 {
		t.Fatalf("code = %d, want 50000", got)
	}

	entries := recorded.All()
	if len(entries) < 2 {
		t.Fatalf("expected panic and request logs, got %d", len(entries))
	}

	panicFound := false
	requestFound := false
	for _, entry := range entries {
		if entry.Message == "panic recovered" {
			panicFound = true
			if traceID := entry.ContextMap()["trace_id"]; traceID != "trace-abc" {
				t.Fatalf("panic log trace_id = %v, want trace-abc", traceID)
			}
		}
		if entry.Message == "http request" {
			requestFound = true
			ctx := entry.ContextMap()
			if traceID := ctx["trace_id"]; traceID != "trace-abc" {
				t.Fatalf("request log trace_id = %v, want trace-abc", traceID)
			}
			if method := ctx["method"]; method != "GET" {
				t.Fatalf("request log method = %v, want GET", method)
			}
			if path := ctx["path"]; path != "/panic" {
				t.Fatalf("request log path = %v, want /panic", path)
			}
			switch headers := ctx["headers"].(type) {
			case map[string]any:
				if got := headers["Authorization"]; got != "***" {
					t.Fatalf("authorization header should be masked, got %v", got)
				}
			case map[string]string:
				if got := headers["Authorization"]; got != "***" {
					t.Fatalf("authorization header should be masked, got %v", got)
				}
			default:
				t.Fatalf("request log headers should be map, got %T", ctx["headers"])
			}
		}
	}

	if !panicFound {
		t.Fatalf("panic recovered log not found")
	}
	if !requestFound {
		t.Fatalf("http request log not found")
	}
}
