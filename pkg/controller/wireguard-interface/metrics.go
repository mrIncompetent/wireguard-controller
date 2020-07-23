package wireguardinterface

import (
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	peerCount prometheus.Gauge
}
