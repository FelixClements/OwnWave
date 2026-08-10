package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ownwave_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ownwave_http_requests_total",
			Help: "Total HTTP requests.",
		},
		[]string{"method", "route", "status"},
	)
)

func init() {
	prometheus.MustRegister(requestDuration, requestCount)
}

func metricsHandler() http.Handler {
	return promhttp.Handler()
}

func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		route := r.URL.Path
		if r.URL.RawPath != "" {
			route = r.URL.RawPath
		}
		next.ServeHTTP(ww, r)
		labels := prometheus.Labels{
			"method": r.Method,
			"route":  route,
			"status": http.StatusText(ww.Status()),
		}
		requestDuration.With(labels).Observe(time.Since(start).Seconds())
		requestCount.With(labels).Inc()
	})
}
