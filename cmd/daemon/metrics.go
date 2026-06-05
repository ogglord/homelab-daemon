// Prometheus /metrics endpoint backed by prometheus/client_golang. Uses a
// dedicated registry (not the default) so we control exactly what's exposed,
// plus the Go runtime + process collectors for free. Bound to a loopback
// port so the only consumer is a Prometheus/VictoriaMetrics scraper on the
// same host.
package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	logging "github.com/ogglord/homelab-logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var metricsLog = logging.Logger("api")

// metricsReg is the dedicated registry. Keeping it package-scoped means
// middleware can record into it without threading the handle through every
// constructor.
var (
	metricsReg = prometheus.NewRegistry()

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests handled by the daemon.",
		},
		// path is intentionally omitted — too high-cardinality. Add a
		// whitelist if per-route data becomes important.
		[]string{"method", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "status"},
	)
)

func init() {
	metricsReg.MustRegister(httpRequestsTotal, httpRequestDuration)
	metricsReg.MustRegister(collectors.NewGoCollector())
	metricsReg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

// recordRequest is called from the daemon API middleware.
func recordRequest(method string, status int, dur time.Duration) {
	statusStr := strconv.Itoa(status)
	httpRequestsTotal.WithLabelValues(method, statusStr).Inc()
	httpRequestDuration.WithLabelValues(method, statusStr).Observe(dur.Seconds())
}

// startMetricsServer launches a Prometheus-compatible /metrics endpoint on
// the given loopback address. Returns when ctx is cancelled.
func startMetricsServer(ctx context.Context, addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metricsReg, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	metricsLog.Info("metrics endpoint listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		metricsLog.Warn("metrics server error", "error", err)
	}
}
