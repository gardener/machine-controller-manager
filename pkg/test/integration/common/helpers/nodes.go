// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"context"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ProbeNodes tries to probe for nodes.
func (c *Cluster) ProbeNodes() error {
	_, err := c.Clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	return err
}

// getNodes tries to retrieve the list of node objects in the cluster.
func (c *Cluster) getNodes() (*corev1.NodeList, error) {
	return c.Clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
}

// GetNumberOfReadyNodes tries to retrieve the list of node objects in the cluster.
func (c *Cluster) GetNumberOfReadyNodes() int16 {
	nodes, _ := c.getNodes()
	count := 0
	for _, n := range nodes.Items {
		for _, nodeCondition := range n.Status.Conditions {
			if nodeCondition.Type == "Ready" && nodeCondition.Status == "True" {
				count++
			}
		}
	}
	return int16(count) //#nosec G115 (CWE-190) -- Test only
}

// This annotation is used by a kwok stage when IT is running in a virtual cluster to perform
// node recovery by setting the `Ready` condition to true after the node lease renewal blocking
// VAP is removed. This is ineffectual for live clusters.
func (c *Cluster) AttemptNodeRecovery(ctx context.Context, nodeName string, attempt int) {
	patch := fmt.Appendf(nil, `{"metadata":{"annotations":{"kwok/fail-condition":"Recover","kwok/recovery-attempts":"%d"}}}`, attempt)
	_, err := c.Clientset.CoreV1().Nodes().Patch(ctx, nodeName, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		log.Printf("AttemptNodeRecovery: failed to patch node %q (attempt %d): %v\n", nodeName, attempt, err)
	}
}

// GetNumberOfNodes returns the total number of node objects in the cluster
func (c *Cluster) GetNumberOfNodes() int16 {
	nodes, _ := c.getNodes()
	return int16(len(nodes.Items)) //#nosec G115 (CWE-190) -- Test only
}
