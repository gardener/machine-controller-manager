// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machineutils

import (
	"testing"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetPreserveAnnotationValue(t *testing.T) {
	tests := []struct {
		name            string
		node            *corev1.Node
		machine         *v1alpha1.Machine
		expectedValue   string
		expectedExists  bool
	}{
		{
			name: "node nil, machine has valid preserve annotation",
			node: nil,
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueWhenFailed,
					},
				},
			},
			expectedValue:  PreserveMachineAnnotationValueWhenFailed,
			expectedExists: true,
		},
		{
			name: "node nil, machine has invalid preserve annotation",
			node: nil,
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: "invalid-value",
					},
				},
			},
			expectedValue:  "",
			expectedExists: false,
		},
		{
			name: "node nil, machine has no preserve annotation",
			node: nil,
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expectedValue:  "",
			expectedExists: false,
		},
		{
			name: "node nil, machine annotations nil",
			node: nil,
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expectedValue:  "",
			expectedExists: false,
		},
		{
			name: "node has valid preserve annotation",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueNow,
					},
				},
			},
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expectedValue:  PreserveMachineAnnotationValueNow,
			expectedExists: true,
		},
		{
			name: "node has invalid preserve annotation, machine has valid preserve annotation",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: "invalid-value",
					},
				},
			},
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueWhenFailed,
					},
				},
			},
			expectedValue:  PreserveMachineAnnotationValueWhenFailed,
			expectedExists: true,
		},
		{
			name: "node has valid preserve annotation, machine also has valid preserve annotation - node takes priority",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueNow,
					},
				},
			},
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueWhenFailed,
					},
				},
			},
			expectedValue:  PreserveMachineAnnotationValueNow,
			expectedExists: true,
		},
		{
			name: "node has no preserve annotation, machine has LastAppliedNodePreserveValue annotation",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LastAppliedNodePreserveValueAnnotationKey: PreserveMachineAnnotationValueNow,
					},
				},
			},
			expectedValue:  "",
			expectedExists: true,
		},
		{
			name: "node has no preserve annotation, machine has no annotations",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expectedValue:  "",
			expectedExists: false,
		},
		{
			name: "node annotations nil, machine has valid preserve annotation",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{},
			},
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueAutoPreserved,
					},
				},
			},
			expectedValue:  PreserveMachineAnnotationValueAutoPreserved,
			expectedExists: true,
		},
		{
			name: "node has false preserve annotation value",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueFalse,
					},
				},
			},
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expectedValue:  PreserveMachineAnnotationValueFalse,
			expectedExists: true,
		},
		{
			name: "node has invalid preserve annotation, machine has LastAppliedNodePreserveValue annotation",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreserveMachineAnnotationKey: "invalid-value",
					},
				},
			},
			machine: &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LastAppliedNodePreserveValueAnnotationKey: PreserveMachineAnnotationValueNow,
					},
				},
			},
			expectedValue:  "",
			expectedExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, exists := GetPreserveAnnotationValue(tt.node, tt.machine)
			if val != tt.expectedValue {
				t.Errorf("expected value %q, got %q", tt.expectedValue, val)
			}
			if exists != tt.expectedExists {
				t.Errorf("expected exists %v, got %v", tt.expectedExists, exists)
			}
		})
	}
}
