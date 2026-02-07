package server

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgarachne_http_requests_total",
			Help: "Total HTTP requests processed by PgArachne.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pgarachne_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	authRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgarachne_auth_requests_total",
			Help: "Authentication attempts by type and result.",
		},
		[]string{"type", "result"},
	)
	loginAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgarachne_login_attempts_total",
			Help: "Login attempts and results.",
		},
		[]string{"result"},
	)
	jsonrpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgarachne_jsonrpc_requests_total",
			Help: "JSON-RPC method calls by result.",
		},
		[]string{"method", "result"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, authRequestsTotal, loginAttemptsTotal, jsonrpcRequestsTotal)
}

func httpMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "__unknown__"
		}
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path, status).Observe(time.Since(start).Seconds())
	}
}

func recordAuthResult(authType, result string) {
	authRequestsTotal.WithLabelValues(authType, result).Inc()
}

func recordLoginResult(result string) {
	loginAttemptsTotal.WithLabelValues(result).Inc()
}

func recordJSONRPC(method, result string) {
	if method == "" {
		method = "unknown"
	}
	if !pgMethodMetricRe.MatchString(method) {
		method = "other"
	}
	jsonrpcRequestsTotal.WithLabelValues(method, result).Inc()
}

func formatStatusResult(status int) string {
	return fmt.Sprintf("%d", status)
}

var pgMethodMetricRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*\.[A-Za-z_][A-Za-z0-9_$]*$`)
