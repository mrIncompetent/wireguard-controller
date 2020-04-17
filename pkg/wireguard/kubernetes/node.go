package kubernetes

import (
	"fmt"
	"net"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
)

const (
	AnnotationKeyPublicKey = "wireguard/public_key"
	AnnotationKeyEndpoint  = "wireguard/endpoint"
)

type PublicKeyNotFoundError struct{}

func (e PublicKeyNotFoundError) Error() string {
	return fmt.Sprintf("No public key could be found in the node's annotation %s", AnnotationKeyPublicKey)
}

func IsPublicKeyNotFound(err error) bool {
	_, isNotFound := err.(PublicKeyNotFoundError)
	return isNotFound
}

func PublicKey(node *corev1.Node) (key wgtypes.Key, err error) {
	sKey := node.Annotations[AnnotationKeyPublicKey]
	if sKey == "" {
		return wgtypes.Key{}, PublicKeyNotFoundError{}
	}

	key, err = wgtypes.ParseKey(sKey)
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("could not parse public key '%s' found in annotation '%s': %w", sKey, AnnotationKeyPublicKey, err)
	}

	return key, nil
}

func SetPublicKey(node *corev1.Node, publicKey wgtypes.Key) bool {
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}

	// We cannot validate public keys :/
	if node.Annotations[AnnotationKeyPublicKey] == "" {
		node.Annotations[AnnotationKeyPublicKey] = publicKey.String()
		return true
	}

	return false
}

type EndpointNotFoundError struct{}

func (e EndpointNotFoundError) Error() string {
	return fmt.Sprintf("No WireGuard endpoint could be found in the node's annotation %s", AnnotationKeyPublicKey)
}

func IsEndpointNotFound(err error) bool {
	_, isNotFound := err.(EndpointNotFoundError)
	return isNotFound
}

func EndpointAddress(node *corev1.Node) (addr *net.UDPAddr, err error) {
	if node.Annotations[AnnotationKeyEndpoint] == "" {
		return nil, EndpointNotFoundError{}
	}

	addr, err = net.ResolveUDPAddr("udp", node.Annotations[AnnotationKeyEndpoint])
	if err != nil {
		return nil, fmt.Errorf("unable to resolve UDP address: %w", err)
	}

	return addr, nil
}

func SetEndpointAddress(node *corev1.Node, address string) bool {
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}

	if node.Annotations[AnnotationKeyEndpoint] != address {
		node.Annotations[AnnotationKeyEndpoint] = address
		return true
	}

	return false
}

type PodCIDRIsEmptyError struct{}

func (e PodCIDRIsEmptyError) Error() string {
	return "Pod CIDR is empty"
}

type Networks []net.IPNet

func (n Networks) String() string {
	var s []string
	for _, network := range n {
		s = append(s, network.String())
	}

	return strings.Join(s, ",")
}

func AllowedNetworks(node *corev1.Node) (networks Networks, err error) {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeExternalIP || addr.Type == corev1.NodeInternalIP {
			ip := net.ParseIP(addr.Address)
			if ip == nil {
				return nil, fmt.Errorf("could not parse IP address: '%s'", addr.Address)
			}

			networks = append(networks, net.IPNet{IP: ip.To4(), Mask: net.IPv4Mask(255, 255, 255, 255)})
		}
	}

	if node.Spec.PodCIDR == "" {
		return nil, PodCIDRIsEmptyError{}
	}

	_, podNet, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		return nil, fmt.Errorf("unable to parse pod CIDR: %w", err)
	}

	networks = append(networks, *podNet)

	return networks, nil
}

func GetPreferredAddress(node *corev1.Node, preferred []corev1.NodeAddressType) *corev1.NodeAddress {
	addresses := map[corev1.NodeAddressType]*corev1.NodeAddress{}

	for idx := range node.Status.Addresses {
		addresses[node.Status.Addresses[idx].Type] = &node.Status.Addresses[idx]
	}

	for _, preferredType := range preferred {
		if addresses[preferredType] != nil {
			return addresses[preferredType]
		}
	}

	return nil
}
