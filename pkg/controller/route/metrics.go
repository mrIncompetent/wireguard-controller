package route

import "github.com/prometheus/client_golang/prometheus"

type metrics struct {
	routeReplaceLatency prometheus.Histogram
}
