package route

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	routeReplaceLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "netlink_route_replace_latency_seconds",
			Help:    "Replace latency in seconds.",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
	)
)

func init() {
	metrics.Registry.MustRegister(routeReplaceLatency)
}
