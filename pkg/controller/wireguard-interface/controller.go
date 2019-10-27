package wireguardinterface

import (
	"context"
	"time"

	"github.com/mrincompetent/wireguard-controller/pkg/source"
	"github.com/mrincompetent/wireguard-controller/pkg/wireguard/kubernetes"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

const (
	name = "wireguard_interface_controller"
)

type keyLoader func() (key *wgtypes.Key, exists bool, err error)

func Add(
	mgr ctrl.Manager,
	log *zap.Logger,
	interfaceName string,
	listeningPort int,
	nodeName string,
	keyLoader keyLoader,
	promRegistry prometheus.Registerer,
) error {
	m := &metrics{
		peerCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_count",
				Help: "Number of configured WireGuard peers.",
			},
		),
	}

	if err := m.Register(promRegistry); err != nil {
		return errors.Wrap(err, "unable to register metrics")
	}

	options := controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &Reconciler{
			Client:        mgr.GetClient(),
			log:           log.Named(name),
			listeningPort: listeningPort,
			interfaceName: interfaceName,
			nodeName:      nodeName,
			loadKey:       keyLoader,
			metrics:       m,
		},
	}

	if err := kubernetes.RegisterPublicKeyIndexer(mgr.GetFieldIndexer()); err != nil {
		return errors.Wrap(err, "unable to register the public key indexer")
	}

	c, err := controller.New(name, mgr, options)
	if err != nil {
		return err
	}

	return c.Watch(source.NewTickerSource(5*time.Second), &handler.EnqueueRequestForObject{})
}

type Reconciler struct {
	client.Client
	log           *zap.Logger
	listeningPort int
	nodeName      string
	interfaceName string
	loadKey       keyLoader
	metrics       *metrics
}

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.log.With(zap.String("sync_id", rand.String(12)))
	log.Debug("Processing")

	var err error

	key, exists, err := r.loadKey()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "unable to load private key")
	}

	if !exists {
		log.Debug("Skipping sync as the private key does not exist yet")
		return ctrl.Result{}, nil
	}

	ownNode := &corev1.Node{}
	if err = r.Client.Get(ctx, types.NamespacedName{Name: r.nodeName}, ownNode); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to load own node")
	}

	if err = r.configureInterface(log, ownNode); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to configure WireGuard interface")
	}

	wgClient, err := wgctrl.New()
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to create a new WireGuard client")
	}

	defer func() {
		if closeErr := wgClient.Close(); closeErr != nil {
			log.Error("unable to close the WireGuard client", zap.Error(closeErr))
		}
	}()

	device, err := wgClient.Device(r.interfaceName)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to get WireGuard interface")
	}

	r.metrics.peerCount.Set(float64(len(device.Peers)))

	interfaceConfig := wgtypes.Config{
		PrivateKey: key,
		ListenPort: &r.listeningPort,
	}

	var reconfigureErrors error

	// Keep the configs indexed by public key
	// That way we know if we already have a peerConfig or need to add a new one
	peerConfigs := map[string]*wgtypes.PeerConfig{}

	// Update & Remove existing peers
	for i := range device.Peers {
		peerLog := log.With(zap.String("peer", device.Peers[i].PublicKey.String()))

		peerConfig, err := kubernetes.PeerConfigForExistingPeer(ctx, peerLog, r.Client, &device.Peers[i])
		if err != nil {
			reconfigureErrors = multierr.Append(
				reconfigureErrors,
				errors.Wrapf(err, "unable to get an updated peer config for peer '%s'", device.Peers[i].PublicKey.String()),
			)

			continue
		}

		peerConfigs[peerConfig.PublicKey.String()] = peerConfig
	}

	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to get WireGuard interface")
	}

	for i := range nodeList.Items {
		nodeLog := log.With(zap.String("node", nodeList.Items[i].Name))

		if nodeList.Items[i].Name == r.nodeName {
			nodeLog.Debug("Skipping node as its the node we're running on")
			continue
		}

		pubKey, err := kubernetes.PublicKey(&nodeList.Items[i])
		if err != nil {
			if kubernetes.IsPublicKeyNotFound(err) {
				nodeLog.Debug("Skipping node as its missing infos: " + err.Error())
			} else {
				reconfigureErrors = multierr.Append(reconfigureErrors, errors.Wrapf(err, "unable to get the public key from node %s", nodeList.Items[i].Name))
			}

			continue
		}

		nodeLog = nodeLog.With(zap.String("public_key", pubKey.String()))

		// If we already have a config for that node, we can skip here
		if _, exists := peerConfigs[pubKey.String()]; exists {
			continue
		}

		peerConfig, err := kubernetes.PeerConfigForNode(log, &nodeList.Items[i])
		if err != nil {
			if kubernetes.IsNodeNotInitializedError(err) {
				nodeLog.Debug("Skipping node: " + err.Error())
				continue
			}

			reconfigureErrors = multierr.Append(reconfigureErrors, errors.Wrapf(err, "unable to build the peer config for node %s", nodeList.Items[i].Name))

			continue
		}

		peerConfigs[peerConfig.PublicKey.String()] = peerConfig

		nodeLog.Info("Added a new peer config")
	}

	for _, peerCfg := range peerConfigs {
		interfaceConfig.Peers = append(interfaceConfig.Peers, *peerCfg)
	}

	if err := wgClient.ConfigureDevice(r.interfaceName, interfaceConfig); err != nil {
		reconfigureErrors = multierr.Append(reconfigureErrors, errors.Wrap(err, "unable to reconfigure interface"))
	}

	return ctrl.Result{}, reconfigureErrors
}
