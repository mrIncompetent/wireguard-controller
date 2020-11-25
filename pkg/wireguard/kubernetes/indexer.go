package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	indexFieldPublicKey = "wireguard-public-key"
)

var ErrGotMultipleNodesWithPublicKey = errors.New("got more than 1 node with the public key. This must not happen")

func publicKeyIndexFunc(o runtime.Object) []string {
	node, ok := o.(*corev1.Node)
	if !ok {
		return nil
	}

	if key := node.Annotations[AnnotationKeyPublicKey]; key != "" {
		return []string{key}
	}

	return nil
}

func RegisterPublicKeyIndexer(ctx context.Context, indexer client.FieldIndexer) error {
	return indexer.IndexField(ctx, &corev1.Node{}, indexFieldPublicKey, publicKeyIndexFunc)
}

func GetNodeByPublicKey(ctx context.Context, c client.Reader, publicKey string) (*corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := c.List(ctx, nodeList, client.MatchingFields{indexFieldPublicKey: publicKey}); err != nil {
		return nil, fmt.Errorf("unable to list nodes: %w", err)
	}

	if len(nodeList.Items) == 0 {
		// Return a NotFound error so we can check that later
		return nil, kerrors.NewNotFound(
			schema.GroupResource{
				Group:    "",
				Resource: "Node",
			},
			publicKey,
		)
	}

	if len(nodeList.Items) > 1 {
		return nil, fmt.Errorf("%w: PublicKey='%s' Nodes:%s", ErrGotMultipleNodesWithPublicKey, publicKey, strings.Join(getNodeNames(nodeList), ","))
	}

	return &nodeList.Items[0], nil
}

func getNodeNames(nodeList *corev1.NodeList) []string {
	names := make([]string, len(nodeList.Items))
	for i := range nodeList.Items {
		names[i] = nodeList.Items[i].Name
	}

	return names
}
