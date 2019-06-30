package wireguard_interface

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	peerCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "wireguard_peer_count",
			Help: "Number of configured WireGuard peers.",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(peerCount)
}
