package wireguardinterface

import (
	"github.com/prometheus/client_golang/prometheus"
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
	prometheus.DefaultRegisterer.MustRegister(peerCount)
}
