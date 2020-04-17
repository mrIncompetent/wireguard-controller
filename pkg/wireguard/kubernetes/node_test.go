package kubernetes

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testhelper "github.com/mrincompetent/wireguard-controller/pkg/test"
)

func nodeWithNetworks(addresses []corev1.NodeAddress, podCidr string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Spec: corev1.NodeSpec{
			PodCIDR: podCidr,
		},
		Status: corev1.NodeStatus{
			Addresses: addresses,
		},
	}
}

func TestAllowedNetworks(t *testing.T) {
	tests := []struct {
		name             string
		node             *corev1.Node
		expectedNetworks Networks
		expectedErr      error
	}{
		{
			name: "valid node addresses",
			node: nodeWithNetworks(
				[]corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: "192.168.1.3",
					},
					{
						Type:    corev1.NodeExternalIP,
						Address: "88.99.100.110",
					},
				},
				"10.244.0.0/24",
			),
			expectedNetworks: []net.IPNet{
				getNet(t, "192.168.1.3/32"),
				getNet(t, "88.99.100.110/32"),
				getNet(t, "10.244.0.0/24"),
			},
		},
		{
			name: "valid node addresses - DNS names get ignored",
			node: nodeWithNetworks(
				[]corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: "192.168.1.3",
					},
					{
						Type:    corev1.NodeExternalDNS,
						Address: "external-dns",
					},
					{
						Type:    corev1.NodeInternalDNS,
						Address: "internal-dns",
					},
					{
						Type:    corev1.NodeHostName,
						Address: "hostname",
					},
				},
				"10.244.0.0/24",
			),
			expectedNetworks: []net.IPNet{
				getNet(t, "192.168.1.3/32"),
				getNet(t, "10.244.0.0/24"),
			},
		},
		{
			name: "no pod CIDR",
			node: nodeWithNetworks(
				[]corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: "192.168.1.3",
					},
				},
				"",
			),
			expectedErr: PodCIDRIsEmptyError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotNetworks, err := AllowedNetworks(test.node)
			testhelper.CompareStrings(t, fmt.Sprint(test.expectedErr), fmt.Sprint(err))
			if err != nil {
				return
			}

			testhelper.CompareStrings(t, test.expectedNetworks.String(), gotNetworks.String())
		})
	}
}

func nodeWithPublicKey(key string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Annotations: map[string]string{
				AnnotationKeyPublicKey: key,
			},
		},
		Spec:   corev1.NodeSpec{},
		Status: corev1.NodeStatus{},
	}
}

func parseKey(t *testing.T, s string) wgtypes.Key {
	key, err := wgtypes.ParseKey(s)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestPublicKey(t *testing.T) {
	tests := []struct {
		name        string
		node        *corev1.Node
		expectedKey wgtypes.Key
		expectedErr error
	}{
		{
			name:        "valid key",
			node:        nodeWithPublicKey("4Uz+l6VDzs4LCwPv4eCuPg2DTROOqjgHF/Ic3lPeYgw="),
			expectedKey: parseKey(t, "4Uz+l6VDzs4LCwPv4eCuPg2DTROOqjgHF/Ic3lPeYgw="),
		},
		{
			name:        "invalid key",
			node:        nodeWithPublicKey("not-valid"),
			expectedErr: errors.New("could not parse public key 'not-valid' found in annotation 'wireguard/public_key': wgtypes: failed to parse base64-encoded key: illegal base64 data at input byte 3"),
		},
		{
			name:        "no key",
			node:        nodeWithPublicKey(""),
			expectedErr: PublicKeyNotFoundError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key, err := PublicKey(test.node)
			testhelper.CompareStrings(t, fmt.Sprint(test.expectedErr), fmt.Sprint(err))
			if err != nil {
				return
			}

			testhelper.CompareStrings(t, test.expectedKey.String(), key.String())
		})
	}
}

func nodeWithEndpoint(endpoint string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Annotations: map[string]string{
				AnnotationKeyEndpoint: endpoint,
			},
		},
		Spec:   corev1.NodeSpec{},
		Status: corev1.NodeStatus{},
	}
}

func TestEndpointAddress(t *testing.T) {
	tests := []struct {
		name            string
		node            *corev1.Node
		expectedAddress string
		expectedErr     error
	}{
		{
			name:            "valid endpoint",
			node:            nodeWithEndpoint("192.168.1.3:51820"),
			expectedAddress: "192.168.1.3:51820",
		},
		{
			name:        "invalid endpoint",
			node:        nodeWithEndpoint("192.168.1.3"),
			expectedErr: errors.New("unable to resolve UDP address: address 192.168.1.3: missing port in address"),
		},
		{
			name:        "no endpoint",
			node:        nodeWithEndpoint(""),
			expectedErr: EndpointNotFoundError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address, err := EndpointAddress(test.node)
			testhelper.CompareStrings(t, fmt.Sprint(test.expectedErr), fmt.Sprint(err))
			if err != nil {
				return
			}

			testhelper.CompareStrings(t, test.expectedAddress, address.String())
		})
	}
}
