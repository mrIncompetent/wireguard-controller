package node

import (
	"context"
	"fmt"
	"time"

	"github.com/mrincompetent/wireguard-controller/pkg/source"
	keyhelper "github.com/mrincompetent/wireguard-controller/pkg/wireguard/key"
	"github.com/mrincompetent/wireguard-controller/pkg/wireguard/kubernetes"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

const (
	name = "node_controller"
)

type Reconciler struct {
	client.Client
	log                *zap.Logger
	nodeName           string
	privateKeyFilePath string
	wireguardPort      int
}

func Add(
	mgr ctrl.Manager,
	log *zap.Logger,
	nodeName,
	privateKeyFilePath string,
	wireGuardPort int,
) error {
	options := controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &Reconciler{
			Client:             mgr.GetClient(),
			log:                log.Named(name),
			nodeName:           nodeName,
			privateKeyFilePath: privateKeyFilePath,
			wireguardPort:      wireGuardPort,
		},
	}
	c, err := controller.New(name, mgr, options)
	if err != nil {
		return err
	}

	return c.Watch(source.NewTickerSource(5*time.Second), &handler.EnqueueRequestForObject{})
}

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.log.With(zap.String("sync_id", rand.String(12)))
	log.Debug("Processing")

	keylog := log.With(zap.String("private_key_file", r.privateKeyFilePath))
	key, err := keyhelper.LoadPrivateKey(r.privateKeyFilePath)
	if err != nil {
		if keyhelper.IsPrivateKeyNotFound(err) {
			keylog.Info("Generating a new private key")
			key, err = keyhelper.GenerateKey(r.privateKeyFilePath)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "unable to generate the private key")
			}
			keylog.Info("Successfully generated a new private key")
		} else {
			return ctrl.Result{}, errors.Wrap(err, "unable to load the private key")
		}
	}

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: r.nodeName}, node); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to load own node")
	}

	if kubernetes.SetPublicKey(node, key.PublicKey()) {
		if err := r.Client.Update(ctx, node); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "unable to store the public key on the node object")
		}
	}

	nodeAddress := kubernetes.GetPreferredAddress(node, []corev1.NodeAddressType{corev1.NodeInternalIP, corev1.NodeExternalIP})
	if nodeAddress == nil {
		return ctrl.Result{}, errors.New(
			"The node, this agent is running on, does not have a usable address. " +
				"Only the following address types can be used: InternalIP, ExternalIP",
		)
	}
	wireGuardEndpoint := fmt.Sprintf("%s:%d", nodeAddress.Address, r.wireguardPort)

	if kubernetes.SetEndpointAddress(node, wireGuardEndpoint) {
		if err := r.Client.Update(ctx, node); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "unable to store the WireGuard endpoint address on the node object")
		}
	}

	return ctrl.Result{}, nil
}
