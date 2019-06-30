package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	AnnotationKeyPublicKey = "wireguard/public_key"
	AnnotationKeyEndpoint  = "wireguard/endpoint"
)

func GetPrivateNodeAddress(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}
