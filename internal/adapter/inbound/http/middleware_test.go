package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPrometheusMiddleware_UnmatchedRouteUsesUnmatchedPathLabel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reg := prometheus.NewRegistry()
	reqTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "test"},
		[]string{"method", "path", "status"},
	)
	reqDur := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "test"},
		[]string{"method", "path", "status"},
	)
	reg.MustRegister(reqTotal, reqDur)

	r := gin.New()
	r.Use(PrometheusMiddleware(reqTotal, reqDur))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/no-such-route", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	ct, err := reqTotal.GetMetricWithLabelValues(http.MethodGet, "/unmatched", "404")
	if err != nil {
		t.Fatal(err)
	}
	if testutil.ToFloat64(ct) != 1 {
		t.Fatalf("expected counter 1 for path=/unmatched, got %v", testutil.ToFloat64(ct))
	}
}
