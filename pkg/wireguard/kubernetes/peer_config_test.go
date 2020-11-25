package kubernetes

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/go-test/deep"
	"go.uber.org/zap/zaptest"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testhelper "github.com/mrincompetent/wireguard-controller/pkg/test"
)

func TestGetPeerConfigForNode(t *testing.T) {
	testPublicKey, err := wgtypes.ParseKey("4Uz+l6VDzs4LCwPv4eCuPg2DTROOqjgHF/Ic3lPeYgw=")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		node            *corev1.Node
		expectedPeerCfg *wgtypes.PeerConfig
		expectedErr     error
	}{
		{
			name: "test with valid node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Annotations: map[string]string{
						AnnotationKeyEndpoint:  "192.168.1.1:51820",
						AnnotationKeyPublicKey: testPublicKey.String(),
					},
				},
				Spec: corev1.NodeSpec{
					PodCIDR: "10.244.0.0/24",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "192.168.1.1",
						},
					},
				},
			},
			expectedPeerCfg: &wgtypes.PeerConfig{
				PublicKey: testPublicKey,
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP("192.168.1.1"),
					Port: 51820,
				},
				AllowedIPs: []net.IPNet{
					getNet(t, "192.168.1.1/32"),
					getNet(t, "10.244.0.0/24"),
				},
			},
		},
		{
			name: "test second node with different pod cidr",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
					Annotations: map[string]string{
						AnnotationKeyEndpoint:  "192.168.1.2:51820",
						AnnotationKeyPublicKey: testPublicKey.String(),
					},
				},
				Spec: corev1.NodeSpec{
					PodCIDR: "10.244.1.0/24",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "192.168.1.2",
						},
					},
				},
			},
			expectedPeerCfg: &wgtypes.PeerConfig{
				PublicKey: testPublicKey,
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP("192.168.1.2"),
					Port: 51820,
				},
				AllowedIPs: []net.IPNet{
					getNet(t, "192.168.1.2/32"),
					getNet(t, "10.244.1.0/24"),
				},
			},
		},
		{
			name: "invalid pod cidr",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node3",
					Annotations: map[string]string{
						AnnotationKeyEndpoint:  "192.168.1.3:51820",
						AnnotationKeyPublicKey: testPublicKey.String(),
					},
				},
				Spec: corev1.NodeSpec{
					PodCIDR: "AAA",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "192.168.1.3",
						},
					},
				},
			},
			expectedErr: errors.New("unable to parse pod CIDR: invalid CIDR address: AAA"),
		},
		{
			name: "invalid WireGuard endpoint",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node4",
					Annotations: map[string]string{
						AnnotationKeyEndpoint:  "AAAA",
						AnnotationKeyPublicKey: testPublicKey.String(),
					},
				},
				Spec: corev1.NodeSpec{
					PodCIDR: "10.244.3.0/24",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "192.168.1.4",
						},
					},
				},
			},
			expectedErr: errors.New("unable to resolve UDP address: address AAAA: missing port in address"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			peerCfg, err := PeerConfigForNode(zaptest.NewLogger(t), test.node)
			testhelper.CompareStrings(t, fmt.Sprint(test.expectedErr), fmt.Sprint(err))
			if test.expectedErr != nil {
				return
			}

			if diff := deep.Equal(test.expectedPeerCfg, peerCfg); diff != nil {
				t.Errorf("got peerCfg does not match the expectedPeerCfg. Diff: \n%v", diff)
			}
		})
	}
}

func getNet(t *testing.T, cidr string) net.IPNet {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", cidr, err)
	}
	return *n
}
