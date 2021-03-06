package node

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/mrincompetent/wireguard-controller/pkg/source"
	"github.com/mrincompetent/wireguard-controller/pkg/wireguard/kubernetes"
)

const (
	name = "node_controller"
)

type KeyStore interface {
	HasKey() bool
	Get() wgtypes.Key
}

type Reconciler struct {
	client.Client
	log           *zap.Logger
	nodeName      string
	wireguardPort int
	keyStore      KeyStore
}

func Add(
	mgr ctrl.Manager,
	log *zap.Logger,
	nodeName string,
	wireGuardPort int,
	keyStore KeyStore,
	metricFactory promauto.Factory,
) error {
	options := controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &Reconciler{
			Client:        mgr.GetClient(),
			log:           log.Named(name),
			nodeName:      nodeName,
			wireguardPort: wireGuardPort,
			keyStore:      keyStore,
		},
	}

	c, err := controller.New(name, mgr, options)
	if err != nil {
		return fmt.Errorf("failed to create new controller: %w", err)
	}

	return c.Watch(source.NewIntervalSource(5*time.Second), &handler.EnqueueRequestForObject{})
}

type nodeAddressTypes []corev1.NodeAddressType

func (addresses nodeAddressTypes) String() string {
	var s []string
	for _, address := range addresses {
		s = append(s, string(address))
	}

	return strings.Join(s, ",")
}

var (
	AllowedNodeAddressTypes = []corev1.NodeAddressType{corev1.NodeInternalIP, corev1.NodeExternalIP}

	ErrNoUsableNodeAddressFound = fmt.Errorf(
		"the node, this agent is running on, does not have a usable address. "+
			"Only the following address types can be used: %s",
		nodeAddressTypes(AllowedNodeAddressTypes).String(),
	)
)

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.log.With(zap.String("sync_id", rand.String(12)))
	log.Debug("Processing")

	if !r.keyStore.HasKey() {
		log.Debug("Requeueing as the private key does not exist yet")

		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
	}

	key := r.keyStore.Get()

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: r.nodeName}, node); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to load own node: %w", err)
	}

	err := retry.OnError(retry.DefaultBackoff, IsConflictError, func() error {
		if err := r.Client.Get(ctx, types.NamespacedName{Name: r.nodeName}, node); err != nil {
			return fmt.Errorf("unable to load own node: %w", err)
		}

		if kubernetes.SetPublicKey(node, key.PublicKey()) {
			if err := r.Client.Update(ctx, node); err != nil {
				return fmt.Errorf("failed to update public key on node: %w", err)
			}
			log.Info("Updated the node's public key")
		}

		return nil
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to store the public key on the node object: %w", err)
	}

	nodeAddress := kubernetes.GetPreferredAddress(node, []corev1.NodeAddressType{corev1.NodeInternalIP, corev1.NodeExternalIP})
	if nodeAddress == nil {
		return ctrl.Result{}, ErrNoUsableNodeAddressFound
	}

	wireGuardEndpoint := fmt.Sprintf("%s:%d", nodeAddress.Address, r.wireguardPort)

	err = retry.OnError(retry.DefaultBackoff, IsConflictError, func() error {
		if err := r.Client.Get(ctx, types.NamespacedName{Name: r.nodeName}, node); err != nil {
			return fmt.Errorf("unable to load own node: %w", err)
		}

		if kubernetes.SetEndpointAddress(node, wireGuardEndpoint) {
			if err := r.Client.Update(ctx, node); err != nil {
				return fmt.Errorf("failed to update endpoint address on node: %w", err)
			}
			log.Info("Updated the node's WireGuard endpoint")
		}

		return nil
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to store the WireGuard endpoint on the node object: %w", err)
	}

	return ctrl.Result{}, nil
}

func IsConflictError(err error) bool {
	var statusErr kerrors.APIStatus
	if errors.As(err, &statusErr) {
		return statusErr.Status().Reason == metav1.StatusReasonConflict
	}

	return false
}
