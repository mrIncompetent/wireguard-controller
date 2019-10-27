package wireguardinterface

import (
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	peerCount prometheus.Gauge
}

func (m *metrics) Register(reg prometheus.Registerer) error {
	return reg.Register(m.peerCount)
}
