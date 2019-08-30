package kubernetes

import (
	"context"
	"net"

	"github.com/go-test/deep"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PeerConfigForNode(log *zap.Logger, node *corev1.Node) (*wgtypes.PeerConfig, error) {
	log = log.Named("peer_config").With(
		zap.String("pod_cidr", node.Spec.PodCIDR),
	)

	key, err := PublicKey(node)
	if err != nil {
		return nil, err
	}
	log = log.With(zap.String("node_public_key", key.String()))
	log.Debug("Parsed the node's WireGuard public key")

	endpoint, err := EndpointAddress(node)
	if err != nil {
		return nil, err
	}
	log = log.With(zap.String("endpoint", endpoint.String()))
	log.Debug("Parsed the node's WireGuard endpoint")

	allowedNetworks, err := AllowedNetworks(node)
	if err != nil {
		return nil, err
	}
	log = log.With(zap.Stringer("allowed_networks", allowedNetworks))
	log.Debug("Determined allowed node networks")

	cfg := wgtypes.PeerConfig{
		Endpoint:   endpoint,
		PublicKey:  key,
		AllowedIPs: allowedNetworks,
	}

	return &cfg, nil
}

func PeerConfigForExistingPeer(ctx context.Context, log *zap.Logger, r client.Reader, peer *wgtypes.Peer) (*wgtypes.PeerConfig, error) {
	pubKey := peer.PublicKey.String()
	cfg := &wgtypes.PeerConfig{
		PublicKey:  peer.PublicKey,
		Endpoint:   peer.Endpoint,
		AllowedIPs: peer.AllowedIPs,
	}

	node, err := GetNodeByPublicKey(ctx, r, pubKey)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// If the node does not exist anymore, delete the peer
			log.Info("Marking peer for removal as the corresponding nodes does not exist anymore")
			cfg.Remove = true
			return cfg, nil
		}
		return nil, errors.Wrapf(err, "unable to get node by public key: %s", pubKey)
	}

	allowedNetworks, err := AllowedNetworks(node)
	if err != nil {
		return nil, err
	}
	log = log.With(zap.Stringer("allowed_networks", allowedNetworks))
	if diff := deep.Equal(cfg.AllowedIPs, []net.IPNet(allowedNetworks)); diff != nil {
		log.Info("Updating the peers allowed networks")
		cfg.AllowedIPs = allowedNetworks
	}

	endpoint, err := EndpointAddress(node)
	if err != nil {
		return nil, err
	}
	log = log.With(zap.String("endpoint", endpoint.String()))
	if cfg.Endpoint.String() != endpoint.String() {
		log.Info("Updating the peers endpoint")
		cfg.Endpoint = endpoint
	}

	return cfg, nil
}
