// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package drain is used to drain nodes
package drain

import corev1 "k8s.io/api/core/v1"

func getPodKey(pod *corev1.Pod) string {
	return pod.Namespace + "/" + pod.Name
}
