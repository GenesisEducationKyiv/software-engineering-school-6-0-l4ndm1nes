package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	GitHubAPICalls      *prometheus.CounterVec
	EmailsSent          *prometheus.CounterVec
	ActiveSubscriptions prometheus.Gauge
	ScanCycles          prometheus.Counter
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
		GitHubAPICalls: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "github_api_calls_total",
				Help: "Total number of GitHub API calls",
			},
			[]string{"endpoint", "status"},
		),
		EmailsSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "emails_sent_total",
				Help: "Total number of emails sent",
			},
			[]string{"type", "status"},
		),
		ActiveSubscriptions: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_subscriptions",
				Help: "Number of active (confirmed) subscriptions",
			},
		),
		ScanCycles: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "scan_cycles_total",
				Help: "Total number of scan cycles completed",
			},
		),
	}

	reg.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.GitHubAPICalls,
		m.EmailsSent,
		m.ActiveSubscriptions,
		m.ScanCycles,
	)

	return m
}
