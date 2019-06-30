package wireguard_interface

import (
	"fmt"
	"net"

	"github.com/mrincompetent/wireguard-controller/pkg/kubernetes"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
)

func GetPeerConfigForNode(parentLog *zap.Logger, node *corev1.Node) (*wgtypes.PeerConfig, error) {
	log := parentLog.Named("peer_config").With(
		zap.String("pod_cidr", node.Spec.PodCIDR),
	)

	pubKey := node.Annotations[kubernetes.AnnotationKeyPublicKey]
	privAddr := kubernetes.GetPrivateNodeAddress(node)
	if privAddr == "" {
		return nil, errors.New("node has no private address")
	}
	log = log.With(zap.String("node_private_address", privAddr))
	log.Debug("Found node's private address")

	key, err := wgtypes.ParseKey(pubKey)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse public key")
	}
	log.Debug("Node has a valid public key")

	endpointAddress := node.Annotations[kubernetes.AnnotationKeyEndpoint]
	if endpointAddress == "" {
		return nil, errors.New("node has no endpoint set")
	}
	log = log.With(zap.String("endpoint", endpointAddress))
	log.Debug("Node has a valid WireGuard endpoint set")

	endpoint, err := net.ResolveUDPAddr("udp", endpointAddress)
	if err != nil {
		return nil, errors.Wrap(err, "unable to resolve UDP address")
	}
	log.Debug("Successfully resolved the node's WireGuard endpoint")

	_, podNet, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse pod CIDR")
	}
	log.Debug("Node has a valid pod CIDR set")

	privNet := fmt.Sprintf("%s/32", privAddr)
	log = log.With(zap.String("node_network", privNet))
	_, nodeNet, err := net.ParseCIDR(privNet)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse node network")
	}

	log.Info("Created the PeerConfig")
	cfg := wgtypes.PeerConfig{
		Endpoint:  endpoint,
		PublicKey: key,
		AllowedIPs: []net.IPNet{
			*podNet,
			*nodeNet,
		},
	}

	return &cfg, nil
}
