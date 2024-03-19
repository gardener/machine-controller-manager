// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *controller) updateNodeWithAnnotations(ctx context.Context, node *v1.Node, annotations map[string]string) error {

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
		klog.Errorf("Failed to update annotations for Node %q due to error: %s", node.Name, err)
		return err
	}

	return nil
}
