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
