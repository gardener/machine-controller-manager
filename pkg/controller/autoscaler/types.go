// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package autoscaler

const (
	// ClusterAutoscalerScaleDownDisabledAnnotationKey annotation to disable the scale-down of the nodes.
	ClusterAutoscalerScaleDownDisabledAnnotationKey = "cluster-autoscaler.kubernetes.io/scale-down-disabled"
	// ClusterAutoscalerScaleDownDisabledAnnotationValue annotation to disable the scale-down of the nodes.
	ClusterAutoscalerScaleDownDisabledAnnotationValue = "true"
	// ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey annotation to disable the scale-down of the nodes.
	ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey = "cluster-autoscaler.kubernetes.io/scale-down-disabled-by-mcm"
	// ClusterAutoscalerScaleDownDisabledAnnotationByMCMValue annotation to disable the scale-down of the nodes.
	ClusterAutoscalerScaleDownDisabledAnnotationByMCMValue = "true"
)
