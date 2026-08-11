package app

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wenlng/go-captcha-service/internal/config"
	"go.uber.org/zap"
)

func TestNewCacheCircuitBreakerUsesServiceName(t *testing.T) {
	const serviceName = "macwk-captcha"

	breaker := newCacheCircuitBreaker(serviceName, zap.NewNop())
	if got := breaker.Name(); got != serviceName {
		t.Fatalf("circuit breaker name = %q, want %q", got, serviceName)
	}
}

func TestMergeWithFlagsCacheKeyPrefix(t *testing.T) {
	const configPrefix = "MACWK_CAPTCHA_DATA:"

	cfg := config.Config{CacheKeyPrefix: configPrefix}
	merged := config.MergeWithFlags(cfg, map[string]interface{}{
		"cache-key-prefix": "",
	})
	assert.Equal(t, configPrefix, merged.CacheKeyPrefix)

	merged = config.MergeWithFlags(cfg, map[string]interface{}{
		"cache-key-prefix": "OVERRIDE_CAPTCHA_DATA:",
	})
	assert.Equal(t, "OVERRIDE_CAPTCHA_DATA:", merged.CacheKeyPrefix)
}

func TestSetupHealthCheck(t *testing.T) {
	t.Run("healthy HTTP and gRPC endpoints", func(t *testing.T) {
		httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/status/health" {
				t.Fatalf("health check path = %q, want %q", request.URL.Path, "/status/health")
			}
			writer.WriteHeader(http.StatusOK)
		}))
		defer httpServer.Close()

		grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen for gRPC health check: %v", err)
		}
		defer grpcListener.Close()

		httpAddr := strings.TrimPrefix(httpServer.URL, "http://")
		if err := setupHealthCheck(httpAddr, grpcListener.Addr().String()); err != nil {
			t.Fatalf("setupHealthCheck() error = %v", err)
		}
	})

	t.Run("unhealthy HTTP status", func(t *testing.T) {
		httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer httpServer.Close()

		httpAddr := strings.TrimPrefix(httpServer.URL, "http://")
		err := setupHealthCheck(httpAddr, "127.0.0.1:1")
		if err == nil || !strings.Contains(err.Error(), "status 503") {
			t.Fatalf("setupHealthCheck() error = %v, want HTTP status error", err)
		}
	})
}
