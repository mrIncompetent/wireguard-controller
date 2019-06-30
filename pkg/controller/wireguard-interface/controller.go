package wireguard_interface

import (
	"context"
	"time"

	"github.com/mrincompetent/wireguard-controller/pkg/kubernetes"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

const (
	name = "wireguard_interface_controller"
)

func Add(
	mgr ctrl.Manager,
	log *zap.Logger,
	interfaceName string,
	wg wgClient,
	key wgtypes.Key,
	listeningPort int,
	ownNodeName string,
	wgAddr *netlink.Addr,
) error {
	options := controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &Reconciler{
			Client:        mgr.GetClient(),
			log:           log.Named(name),
			wg:            wg,
			key:           key,
			listeningPort: listeningPort,
			interfaceName: interfaceName,
			ownNodeName:   ownNodeName,
			wgAddr:        wgAddr,
		},
	}
	c, err := controller.New(name, mgr, options)
	if err != nil {
		return err
	}

	return c.Watch(kubernetes.NewTickerSource(5*time.Second), &handler.EnqueueRequestForObject{})
}

type wgClient interface {
	ConfigureDevice(name string, cfg wgtypes.Config) error
	Device(name string) (*wgtypes.Device, error)
}

type Reconciler struct {
	client.Client
	log           *zap.Logger
	wg            wgClient
	key           wgtypes.Key
	listeningPort int
	ownNodeName   string
	interfaceName string
	wgAddr        *netlink.Addr
}

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.log.With()
	log.Debug("Processing")

	nodeList := &corev1.NodeList{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Client.List(ctx, nodeList); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to list nodes")
	}

	if err := r.configureInterface(log); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to configure WireGuard interface")
	}

	device, err := r.wg.Device(r.interfaceName)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to get WireGuard interface")
	}
	peerCount.Set(float64(len(device.Peers)))

	// Build up peer info by ip for faster lookups below
	peers := map[string]*wgtypes.Peer{}
	peerSet := sets.NewString()
	for i, peer := range device.Peers {
		pubKey := peer.PublicKey.String()
		peerSet.Insert(pubKey)
		peers[pubKey] = &device.Peers[i]
	}

	nodes := map[string]*corev1.Node{}
	nodeSet := sets.NewString()
	for i, node := range nodeList.Items {
		pubKey := node.Annotations[kubernetes.AnnotationKeyPublicKey]
		if pubKey == "" {
			log.Debug("Skipping node as it has no public key set", zap.String("node", node.Name))
			continue
		}
		if node.Name == r.ownNodeName {
			continue
		}
		nodeSet.Insert(pubKey)
		nodes[pubKey] = &nodeList.Items[i]
	}

	interfaceConfig := wgtypes.Config{
		PrivateKey: &r.key,
		ListenPort: &r.listeningPort,
	}

	// Check if we have any peers which have no matching node object
	if diff := peerSet.Difference(nodeSet); diff.Len() > 0 {
		// Delete configured WireGuard peers
		for _, pubKey := range diff.List() {
			peer := peers[pubKey]
			log.Info("Removing peer as it has no associated node", zap.String("peer", pubKey))

			peerCfg := wgtypes.PeerConfig{
				PublicKey: peer.PublicKey,
				Remove:    true,
			}
			interfaceConfig.Peers = append(interfaceConfig.Peers, peerCfg)
		}
	}

	// We collect all errors in a slice to return them later.
	// We want to process as much as possible. Returning early with an error can break the cluster network
	var combinedErr error

	// Check if we have any nodes which have no configured WireGuard peer
	if diff := nodeSet.Difference(peerSet); diff.Len() > 0 {
		// Add the WireGuard peer
		for _, pubKey := range diff.List() {
			node := nodes[pubKey]
			nodeLog := log.With(
				zap.String("node", node.Name),
				zap.String("public_key", pubKey),
			)

			peerCfg, err := GetPeerConfigForNode(nodeLog, node)
			if err != nil {
				combinedErr = multierr.Append(combinedErr, errors.WithMessagef(err, "unable to create the peer config for node '%s'", node.Name))
				continue
			}

			nodeLog.Info("Successfully added node to config")
			interfaceConfig.Peers = append(interfaceConfig.Peers, *peerCfg)
		}
	}

	if err := r.wg.ConfigureDevice(r.interfaceName, interfaceConfig); err != nil {
		combinedErr = multierr.Append(combinedErr, errors.Wrap(err, "unable to reconfigure interface"))
	}

	if combinedErr != nil {
		return ctrl.Result{}, combinedErr
	}

	return ctrl.Result{}, nil
}
