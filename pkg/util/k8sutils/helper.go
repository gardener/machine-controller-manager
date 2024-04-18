// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package k8sutils is used to provider helper consts and functions for k8s operations
package k8sutils

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
