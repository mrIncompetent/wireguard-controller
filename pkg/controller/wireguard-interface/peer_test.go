package wireguard_interface

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/mrincompetent/wireguard-controller/pkg/kubernetes"
	customlog "github.com/mrincompetent/wireguard-controller/pkg/log"
	testhelper "github.com/mrincompetent/wireguard-controller/pkg/test"

	"github.com/go-test/deep"
	"go.uber.org/zap/zapcore"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetPeerConfigForNode(t *testing.T) {
	const testPubKey = "4Uz+l6VDzs4LCwPv4eCuPg2DTROOqjgHF/Ic3lPeYgw="
	testKey, err := wgtypes.ParseKey(testPubKey)
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
						kubernetes.AnnotationKeyEndpoint:  "192.168.1.1:51820",
						kubernetes.AnnotationKeyPublicKey: testKey.String(),
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
				PublicKey: testKey,
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP("192.168.1.1"),
					Port: 51820,
				},
				AllowedIPs: []net.IPNet{
					*getNet(t, "10.244.0.0/24"),
					*getNet(t, "192.168.1.1/32"),
				},
			},
			expectedLog: `debug	peer_config	Found node's private address	{"pod_cidr": "10.244.0.0/24", "node_private_address": "192.168.1.1"}
debug	peer_config	Node has a valid public key	{"pod_cidr": "10.244.0.0/24", "node_private_address": "192.168.1.1"}
debug	peer_config	Node has a valid WireGuard endpoint set	{"pod_cidr": "10.244.0.0/24", "node_private_address": "192.168.1.1", "endpoint": "192.168.1.1:51820"}
debug	peer_config	Successfully resolved the node's WireGuard endpoint	{"pod_cidr": "10.244.0.0/24", "node_private_address": "192.168.1.1", "endpoint": "192.168.1.1:51820"}
debug	peer_config	Node has a valid pod CIDR set	{"pod_cidr": "10.244.0.0/24", "node_private_address": "192.168.1.1", "endpoint": "192.168.1.1:51820"}
info	peer_config	Created the PeerConfig	{"pod_cidr": "10.244.0.0/24", "node_private_address": "192.168.1.1", "endpoint": "192.168.1.1:51820", "node_network": "192.168.1.1/32"}
`,
		},
		{
			name: "test second node with different pod cidr",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
					Annotations: map[string]string{
						kubernetes.AnnotationKeyEndpoint:  "192.168.1.2:51820",
						kubernetes.AnnotationKeyPublicKey: testKey.String(),
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
				PublicKey: testKey,
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP("192.168.1.2"),
					Port: 51820,
				},
				AllowedIPs: []net.IPNet{
					*getNet(t, "10.244.1.0/24"),
					*getNet(t, "192.168.1.2/32"),
				},
			},
			expectedLog: `debug	peer_config	Found node's private address	{"pod_cidr": "10.244.1.0/24", "node_private_address": "192.168.1.2"}
debug	peer_config	Node has a valid public key	{"pod_cidr": "10.244.1.0/24", "node_private_address": "192.168.1.2"}
debug	peer_config	Node has a valid WireGuard endpoint set	{"pod_cidr": "10.244.1.0/24", "node_private_address": "192.168.1.2", "endpoint": "192.168.1.2:51820"}
debug	peer_config	Successfully resolved the node's WireGuard endpoint	{"pod_cidr": "10.244.1.0/24", "node_private_address": "192.168.1.2", "endpoint": "192.168.1.2:51820"}
debug	peer_config	Node has a valid pod CIDR set	{"pod_cidr": "10.244.1.0/24", "node_private_address": "192.168.1.2", "endpoint": "192.168.1.2:51820"}
info	peer_config	Created the PeerConfig	{"pod_cidr": "10.244.1.0/24", "node_private_address": "192.168.1.2", "endpoint": "192.168.1.2:51820", "node_network": "192.168.1.2/32"}
`,
		},
		{
			name: "invalid pod cidr",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node3",
					Annotations: map[string]string{
						kubernetes.AnnotationKeyEndpoint:  "192.168.1.3:51820",
						kubernetes.AnnotationKeyPublicKey: testKey.String(),
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
						kubernetes.AnnotationKeyEndpoint:  "AAAA",
						kubernetes.AnnotationKeyPublicKey: testKey.String(),
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
			defer log.Sync()

			peerCfg, err := GetPeerConfigForNode(log, test.node)
			if fmt.Sprint(err) != fmt.Sprint(test.expectedErr) {
				t.Error(err)
			}
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

func getNet(t *testing.T, cidr string) *net.IPNet {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", cidr, err)
	}
	return n
}
