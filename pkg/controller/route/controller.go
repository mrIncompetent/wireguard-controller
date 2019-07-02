package route

import (
	"context"
	"net"
	"time"

	"github.com/mrincompetent/wireguard-controller/pkg/kubernetes"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
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
}

func Add(
	mgr ctrl.Manager,
	log *zap.Logger,
	interfaceName string,
	nodeName string,
) error {
	options := controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &Reconciler{
			Client:        mgr.GetClient(),
			log:           log.Named(name),
			interfaceName: interfaceName,
			nodeName:      nodeName,
		},
	}
	c, err := controller.New(name, mgr, options)
	if err != nil {
		return err
	}

	return c.Watch(kubernetes.NewTickerSource(5*time.Second), &handler.EnqueueRequestForObject{})
}

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.log.With()
	log.Debug("Processing")

	link, err := netlink.LinkByName(r.interfaceName)
	if err != nil {
		// In case the interface was not created yet we requeue
		if _, isNotFound := err.(netlink.LinkNotFoundError); isNotFound {
			log.Debug("Skipping route reconciling since the link is not up yet")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "unable to get interface details")
	}

	nodeList := &corev1.NodeList{}
	if err := r.List(context.Background(), nodeList); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to list nodes")
	}

	var combinedErr error
	for _, node := range nodeList.Items {
		if node.Name == r.nodeName {
			// Do not setup routes for the local node.
			continue
		}

		nodeLog := log.With(zap.String("node", node.Name))
		if err := r.setupRoute(nodeLog, link, node); err != nil {
			combinedErr = multierr.Append(combinedErr, errors.WithMessagef(err, "unable to setup route for node '%s'", node.Name))
			continue
		}
	}

	if combinedErr != nil {
		return ctrl.Result{}, combinedErr
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) setupRoute(log *zap.Logger, link netlink.Link, node corev1.Node) error {
	_, podCIDRNet, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		return errors.Wrap(err, "unable to parse pod CIDR")
	}

	route := netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       podCIDRNet,
		Table:     254,
	}

	start := time.Now()
	if err := netlink.RouteReplace(&route); err != nil {
		return errors.Wrap(err, "unable to replace route")
	}
	routeReplaceLatency.Observe(time.Since(start).Seconds())

	log.Debug("Replaced route", zap.String("route", route.String()))

	return nil
}
