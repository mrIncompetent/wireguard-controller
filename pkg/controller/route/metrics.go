package route

import "github.com/prometheus/client_golang/prometheus"

type metrics struct {
	routeReplaceLatency prometheus.Histogram
}

func (m *metrics) Register(reg prometheus.Registerer) error {
	return reg.Register(m.routeReplaceLatency)
}
