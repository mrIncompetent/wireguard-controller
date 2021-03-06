package wireguardinterface

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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

	"github.com/mrincompetent/wireguard-controller/pkg/source"
	"github.com/mrincompetent/wireguard-controller/pkg/wireguard/kubernetes"
)

const (
	name = "wireguard_interface_controller"
)

func Add(
	ctx context.Context,
	mgr ctrl.Manager,
	log *zap.Logger,
	interfaceName string,
	listeningPort int,
	nodeName string,
	keyStore KeyStore,
	metricFactory promauto.Factory,
) error {
	m := &metrics{
		peerCount: metricFactory.NewGauge(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_count",
				Help: "Number of configured WireGuard peers.",
			},
		),
	}

	options := controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &Reconciler{
			Client:        mgr.GetClient(),
			log:           log.Named(name),
			listeningPort: listeningPort,
			interfaceName: interfaceName,
			nodeName:      nodeName,
			keyStore:      keyStore,
			metrics:       m,
		},
	}

	if err := kubernetes.RegisterPublicKeyIndexer(ctx, mgr.GetFieldIndexer()); err != nil {
		return fmt.Errorf("unable to register the public key indexer: %w", err)
	}

	c, err := controller.New(name, mgr, options)
	if err != nil {
		return fmt.Errorf("failed to create new controller: %w", err)
	}

	return c.Watch(source.NewIntervalSource(5*time.Second), &handler.EnqueueRequestForObject{})
}

type KeyStore interface {
	HasKey() bool
	Get() wgtypes.Key
}

type Reconciler struct {
	client.Client
	log           *zap.Logger
	listeningPort int
	nodeName      string
	interfaceName string
	metrics       *metrics
	keyStore      KeyStore
}

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.log.With(zap.String("sync_id", rand.String(12)))
	log.Debug("Processing")

	var err error

	if !r.keyStore.HasKey() {
		log.Debug("Requeueing as the private key does not exist yet")

		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
	}

	key := r.keyStore.Get()

	ownNode := &corev1.Node{}
	if err = r.Client.Get(ctx, types.NamespacedName{Name: r.nodeName}, ownNode); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to load own node: %w", err)
	}

	if err = r.configureInterface(log, ownNode); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to configure WireGuard interface: %w", err)
	}

	wgClient, err := wgctrl.New()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to create a new WireGuard client: %w", err)
	}

	defer func() {
		if closeErr := wgClient.Close(); closeErr != nil {
			log.Error("unable to close the WireGuard client", zap.Error(closeErr))
		}
	}()

	device, err := wgClient.Device(r.interfaceName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get WireGuard interface: %w", err)
	}

	r.metrics.peerCount.Set(float64(len(device.Peers)))

	interfaceConfig := wgtypes.Config{
		PrivateKey: &key,
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
				fmt.Errorf("unable to get an updated peer config for peer '%s': %w", device.Peers[i].PublicKey.String(), err),
			)

			continue
		}

		peerConfigs[peerConfig.PublicKey.String()] = peerConfig
	}

	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get WireGuard interface: %w", err)
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
				reconfigureErrors = multierr.Append(reconfigureErrors, fmt.Errorf("unable to get the public key from node %s: %w", nodeList.Items[i].Name, err))
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

			reconfigureErrors = multierr.Append(reconfigureErrors, fmt.Errorf("unable to build the peer config for node %s: %w", nodeList.Items[i].Name, err))

			continue
		}

		peerConfigs[peerConfig.PublicKey.String()] = peerConfig

		nodeLog.Info("Added a new peer config")
	}

	for _, peerCfg := range peerConfigs {
		interfaceConfig.Peers = append(interfaceConfig.Peers, *peerCfg)
	}

	if err := wgClient.ConfigureDevice(r.interfaceName, interfaceConfig); err != nil {
		reconfigureErrors = multierr.Append(reconfigureErrors, fmt.Errorf("unable to reconfigure interface: %w", err))
	}

	if reconfigureErrors != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconfigure at least one node: %w", err)
	}

	return ctrl.Result{}, nil
}
