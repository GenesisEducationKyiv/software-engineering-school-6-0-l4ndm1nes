package http

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type RouterConfig struct {
	Handler         *Handler
	APIKey          string
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
}

func NewRouter(cfg RouterConfig) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	if cfg.RequestsTotal != nil && cfg.RequestDuration != nil {
		r.Use(PrometheusMiddleware(cfg.RequestsTotal, cfg.RequestDuration))
	}

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/api/swagger.yaml", serveSwaggerYAML)

	webDir := resolveWebDir()
	r.Static("/static", webDir)
	r.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(webDir, "index.html"))
	})

	api := r.Group("/api")
	if cfg.APIKey != "" {
		api.Use(APIKeyAuth(cfg.APIKey))
	}
	api.POST("/subscribe", cfg.Handler.Subscribe)
	api.GET("/confirm/:token", cfg.Handler.Confirm)
	api.GET("/unsubscribe/:token", cfg.Handler.Unsubscribe)
	api.GET("/subscriptions", cfg.Handler.GetSubscriptions)

	return r
}

func resolveWebDir() string {
	if dir := os.Getenv("WEB_DIR"); dir != "" {
		return dir
	}
	return "./web"
}

func serveSwaggerYAML(c *gin.Context) {
	path := os.Getenv("SWAGGER_PATH")
	if path == "" {
		path = filepath.Join("api", "swagger.yaml")
	}
	c.Header("Content-Type", "application/yaml; charset=utf-8")
	c.File(path)
}
