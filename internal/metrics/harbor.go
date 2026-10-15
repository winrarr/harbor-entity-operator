package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	harborAPIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "harbor_operator_harbor_api_requests_total",
			Help: "Total number of Harbor API requests made by the operator.",
		},
		[]string{"method", "endpoint", "status"},
	)

	harborAPIRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "harbor_operator_harbor_api_request_duration_seconds",
			Help:    "Duration of Harbor API requests made by the operator.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status"},
	)

	harborHTTPClientCacheEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "harbor_operator_harbor_http_client_cache_events_total",
			Help: "Harbor HTTP client cache events, with event set to hit, miss, invalidation, or eviction.",
		},
		[]string{"event"},
	)
)

func init() {
	prometheus.MustRegister(harborAPIRequestsTotal, harborAPIRequestDurationSeconds)
	prometheus.MustRegister(harborHTTPClientCacheEvents)
}

// ObserveHarborHTTPClientCache records a bounded-cardinality cache event.
func ObserveHarborHTTPClientCache(event string) {
	harborHTTPClientCacheEvents.WithLabelValues(event).Inc()
}

// ObserveHarborRequest records a single Harbor API request.
func ObserveHarborRequest(method, endpoint string, status int, durationSeconds float64) {
	statusLabel := "error"
	if status > 0 {
		statusLabel = strconv.Itoa(status)
	}
	harborAPIRequestsTotal.WithLabelValues(method, endpoint, statusLabel).Inc()
	harborAPIRequestDurationSeconds.WithLabelValues(method, endpoint, statusLabel).Observe(durationSeconds)
}
