package kubernetes

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"testing"

	customlog "github.com/mrincompetent/wireguard-controller/pkg/log"
	testhelper "github.com/mrincompetent/wireguard-controller/pkg/test"

	"github.com/go-test/deep"
	"go.uber.org/zap/zapcore"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		expectedLog     string
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
			expectedLog: `debug	peer_config	Parsed the node's WireGuard public key	{"pod_cidr": "10.244.0.0/24", "node_public_key": "4Uz+l6VDzs4LCwPv4eCuPg2DTROOqjgHF/Ic3lPeYgw="}
debug	peer_config	Parsed the node's WireGuard endpoint	{"pod_cidr": "10.244.0.0/24", "node_public_key": "4Uz+l6VDzs4LCwPv4eCuPg2DTROOqjgHF/Ic3lPeYgw=", "endpoint": "192.168.1.1:51820"}
debug	peer_config	Determined allowed node networks	{"pod_cidr": "10.244.0.0/24", "node_public_key": "4Uz+l6VDzs4LCwPv4eCuPg2DTROOqjgHF/Ic3lPeYgw=", "endpoint": "192.168.1.1:51820", "allowed_networks": "192.168.1.1/32,10.244.0.0/24"}
`,
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
			expectedLog: `debug	peer_config	Parsed the node's WireGuard public key	{"pod_cidr": "10.244.1.0/24", "node_public_key": "4Uz+l6VDzs4LCwPv4eCuPg2DTROOqjgHF/Ic3lPeYgw="}
debug	peer_config	Parsed the node's WireGuard endpoint	{"pod_cidr": "10.244.1.0/24", "node_public_key": "4Uz+l6VDzs4LCwPv4eCuPg2DTROOqjgHF/Ic3lPeYgw=", "endpoint": "192.168.1.2:51820"}
debug	peer_config	Determined allowed node networks	{"pod_cidr": "10.244.1.0/24", "node_public_key": "4Uz+l6VDzs4LCwPv4eCuPg2DTROOqjgHF/Ic3lPeYgw=", "endpoint": "192.168.1.2:51820", "allowed_networks": "192.168.1.2/32,10.244.1.0/24"}
`,
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
			expectedLog: ``,
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
			expectedLog: ``,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logOutput := &bytes.Buffer{}
			log := customlog.NewTestLog(zapcore.AddSync(logOutput))
			defer t.Log(logOutput.String())
			defer log.Sync()

			peerCfg, err := PeerConfigForNode(log, test.node)
			testhelper.CompareStrings(t, fmt.Sprint(test.expectedErr), fmt.Sprint(err))
			if test.expectedErr != nil {
				return
			}

			if diff := deep.Equal(test.expectedPeerCfg, peerCfg); diff != nil {
				t.Errorf("got peerCfg does not match the expectedPeerCfg. Diff: \n%v", diff)
			}

			// Test log output
			log.Sync()
			testhelper.CompareStrings(t, test.expectedLog, logOutput.String())
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
