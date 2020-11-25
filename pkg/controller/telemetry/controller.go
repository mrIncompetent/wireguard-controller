package telemetry

import (
	"context"
	"errors"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/heptiolabs/healthcheck"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	name = "health_handler"
)

type healthHandler struct {
	server *http.Server
	log    *zap.Logger
}

func Add(mgr ctrl.Manager, log *zap.Logger, promRegistry prometheus.Gatherer, listenAddress string) error {
	health := healthcheck.NewMetricsHandler(metrics.Registry, "wireguard_controller")
	router := http.NewServeMux()

	registries := prometheus.Gatherers{
		promRegistry,
		metrics.Registry,
	}

	// Metrics
	router.Handle("/metrics", promhttp.HandlerFor(registries, promhttp.HandlerOpts{Timeout: 5 * time.Second}))

	// Liveness / Readiness
	router.HandleFunc("/live", health.LiveEndpoint)
	router.HandleFunc("/ready", health.ReadyEndpoint)

	// PProf
	router.HandleFunc("/debug/pprof/", pprof.Index)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)

	h := &healthHandler{
		log: log.Named(name).With(zap.String("listen-address", listenAddress)),
		server: &http.Server{
			Addr:         listenAddress,
			Handler:      router,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  15 * time.Second,
		},
	}

	return mgr.Add(h)
}

func (h *healthHandler) Start(stop <-chan struct{}) error {
	go func() {
		h.log.Info("Starting the telemetry server")

		if err := h.server.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				h.log.Error("Failed to start the http server", zap.Error(err))
			}
		}
	}()

	<-stop
	h.log.Info("Stopping the telemetry server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return h.server.Shutdown(ctx)
}
