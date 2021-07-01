package controller

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *controller) updateNodeWithAnnotation(ctx context.Context, node *v1.Node, annotations map[string]string) error {

	// Initialize node annotations if empty
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	//append annotations
	for k, v := range annotations {
		node.Annotations[k] = v
	}

	_, err := c.targetCoreClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})

	if err != nil {
		klog.Errorf("Couldn't patch the node %q , Error: %s", node.Name, err)
		return err
	}
	klog.V(2).Infof("Annotated node %q was annotated with NotManagedByMCM successfully", node.Name)

	return nil
}
