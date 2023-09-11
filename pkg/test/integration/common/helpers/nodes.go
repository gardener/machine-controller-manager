// Copyright 2023 SAP SE or an SAP affiliate company
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProbeNodes tries to probe for nodes.
func (c *Cluster) ProbeNodes() error {
	_, err := c.Clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	return err
}

// getNodes tries to retrieve the list of node objects in the cluster.
func (c *Cluster) getNodes() (*v1.NodeList, error) {
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
	return int16(count)
}

// GetNumberOfNodes tries to retrieve the list of node objects in the cluster.
func (c *Cluster) GetNumberOfNodes() int16 {
	nodes, _ := c.getNodes()
	return int16(len(nodes.Items))
}
