package route

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/mrincompetent/wireguard-controller/pkg/source"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vishvananda/netlink"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

const (
	name = "route_controller"
)

type Reconciler struct {
	client.Client
	log           *zap.Logger
	interfaceName string
	nodeName      string
	metrics       *metrics
}

func Add(
	mgr ctrl.Manager,
	log *zap.Logger,
	interfaceName,
	nodeName string,
	metricFactory promauto.Factory,
) error {
	m := &metrics{
		routeReplaceLatency: metricFactory.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "netlink_route_replace_latency_seconds",
				Help:    "Replace latency in seconds.",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
			},
		),
	}

	options := controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &Reconciler{
			Client:        mgr.GetClient(),
			log:           log.Named(name),
			interfaceName: interfaceName,
			nodeName:      nodeName,
			metrics:       m,
		},
	}

	c, err := controller.New(name, mgr, options)
	if err != nil {
		return err
	}

	return c.Watch(source.NewIntervalSource(5*time.Second), &handler.EnqueueRequestForObject{})
}

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.log.With(zap.String("sync_id", rand.String(12)))
	log.Debug("Processing")

	link, err := netlink.LinkByName(r.interfaceName)
	if err != nil {
		// In case the interface was not created yet we requeue
		if _, isNotFound := err.(netlink.LinkNotFoundError); isNotFound {
			log.Debug("Skipping route reconciling since the link is not up yet")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("unable to get interface details: %w", err)
	}

	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to list nodes: %w", err)
	}

	var combinedErr error

	for i := range nodeList.Items {
		if nodeList.Items[i].Name == r.nodeName {
			// Do not setup routes for the local node.
			continue
		}

		nodeLog := log.With(zap.String("node", nodeList.Items[i].Name))
		if err := r.setupRoute(nodeLog, link, &nodeList.Items[i]); err != nil {
			combinedErr = multierr.Append(combinedErr, fmt.Errorf("unable to setup route for node '%s': %w", nodeList.Items[i].Name, err))
			continue
		}
	}

	if combinedErr != nil {
		return ctrl.Result{}, combinedErr
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) setupRoute(log *zap.Logger, link netlink.Link, node *corev1.Node) error {
	_, podCIDRNet, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		return fmt.Errorf("unable to parse pod CIDR: %w", err)
	}

	route := netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       podCIDRNet,
		Table:     254,
	}

	start := time.Now()

	if err := netlink.RouteReplace(&route); err != nil {
		return fmt.Errorf("unable to replace route: %w", err)
	}

	r.metrics.routeReplaceLatency.Observe(time.Since(start).Seconds())

	log.Debug("Replaced route", zap.String("route", route.String()))

	return nil
}
