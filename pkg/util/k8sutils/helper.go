/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package k8sutils is used to provider helper consts and functions for k8s operations
package k8sutils

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

const (
	// VolumeAttachmentGroupName group name
	VolumeAttachmentGroupName = "storage.k8s.io"
	// VolumeAttachmentResourceName is the kind used for VolumeAttachment
	VolumeAttachmentResourceName = "volumeattachments"
)

// IsResourceSupported uses Discovery API to find out if the server supports
// the given GroupResource.
// If supported, it will return its groupVersion; Otherwise, it will return ""
func IsResourceSupported(
	clientset kubernetes.Interface,
	gr schema.GroupResource,
) bool {
	var (
		foundDesiredGroup   bool
		desiredGroupVersion string
	)

	discoveryClient := clientset.Discovery()
	groupList, err := discoveryClient.ServerGroups()
	if err != nil {
		return false
	}

	for _, group := range groupList.Groups {
		if group.Name == gr.Group {
			foundDesiredGroup = true
			desiredGroupVersion = group.PreferredVersion.GroupVersion
			break
		}
	}
	if !foundDesiredGroup {
		return false
	}

	resourceList, err := discoveryClient.ServerResourcesForGroupVersion(desiredGroupVersion)
	if err != nil {
		return false
	}

	for _, resource := range resourceList.APIResources {
		if resource.Name == gr.Resource {
			klog.V(3).Infof("Found Resource: %s/%s", gr.Group, gr.Resource)
			return true
		}
	}
	return false
}
