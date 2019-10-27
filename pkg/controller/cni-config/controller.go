package cniconfig

import (
	"context"
	"net"
	"time"

	"github.com/mrincompetent/wireguard-controller/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

const (
	name = "cni_config_controller"
)

func Add(
	mgr ctrl.Manager,
	log *zap.Logger,
	cniTemplateDir,
	cniConfigPath,
	interfaceName string,
	podNet *net.IPNet,
	nodeName string,
) error {
	options := controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &Reconciler{
			Client:        mgr.GetClient(),
			log:           log.Named(name),
			interfaceName: interfaceName,
			nodeName:      nodeName,
			podNet:        podNet,
			cni: CNIConfig{
				TargetDir:   cniConfigPath,
				TemplateDir: cniTemplateDir,
			},
		},
	}

	c, err := controller.New(name, mgr, options)
	if err != nil {
		return err
	}

	return c.Watch(source.NewTickerSource(5*time.Second), &handler.EnqueueRequestForObject{})
}

type CNIConfig struct {
	TemplateDir string
	TargetDir   string
}

type Reconciler struct {
	client.Client
	log           *zap.Logger
	cni           CNIConfig
	interfaceName string
	podNet        *net.IPNet
	nodeName      string
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

		return ctrl.Result{}, errors.Wrap(err, "unable to get interface details")
	}

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: r.nodeName}, node); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to load own node")
	}

	if err := r.writeCNIConfig(log, node, link.Attrs().MTU); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to write CNI config")
	}

	return ctrl.Result{}, nil
}
